// Package provider is a plugin to use a proxmox cluster as an provider.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NX211/traefik-proxmox-provider/internal"
	"github.com/traefik/genconf/dynamic"
	"github.com/traefik/genconf/dynamic/tls"
	"github.com/traefik/genconf/dynamic/types"
)

// Config the plugin configuration.
type Config struct {
	PollInterval   string `json:"pollInterval" yaml:"pollInterval" toml:"pollInterval"`
	ApiEndpoint    string `json:"apiEndpoint" yaml:"apiEndpoint" toml:"apiEndpoint"`
	ApiTokenId     string `json:"apiTokenId" yaml:"apiTokenId" toml:"apiTokenId"`
	ApiToken       string `json:"apiToken" yaml:"apiToken" toml:"apiToken"`
	ApiLogging     string `json:"apiLogging" yaml:"apiLogging" toml:"apiLogging"`
	ApiValidateSSL string `json:"apiValidateSSL" yaml:"apiValidateSSL" toml:"apiValidateSSL"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		PollInterval:   "30s", // Default to 30 seconds for polling
		ApiValidateSSL: "true",
		ApiLogging:     "info",
	}
}

// Provider a plugin.
type Provider struct {
	name         string
	pollInterval time.Duration
	client       *internal.ProxmoxClient
	cancel       func()
}

// New creates a new Provider plugin.
func New(ctx context.Context, config *Config, name string) (*Provider, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	pi, err := time.ParseDuration(config.PollInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid poll interval: %w", err)
	}

	// Ensure minimum poll interval
	if pi < 5*time.Second {
		return nil, fmt.Errorf("poll interval must be at least 5 seconds, got %v", pi)
	}

	pc, err := newParserConfig(
		config.ApiEndpoint,
		config.ApiTokenId,
		config.ApiToken,
		config.ApiLogging,
		config.ApiValidateSSL == "true",
	)
	if err != nil {
		return nil, fmt.Errorf("invalid parser config: %w", err)
	}
	client := newClient(pc)

	if err := logVersion(client, ctx); err != nil {
		return nil, fmt.Errorf("failed to get Proxmox version: %w", err)
	}

	return &Provider{
		name:         name,
		pollInterval: pi,
		client:       client,
	}, nil
}

// Init the provider.
func (p *Provider) Init() error {
	return nil
}

// Provide creates and send dynamic configuration.
func (p *Provider) Provide(cfgChan chan<- json.Marshaler) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Recovered from panic in provider: %v", err)
			}
		}()

		p.loadConfiguration(ctx, cfgChan)
	}()

	return nil
}

func (p *Provider) loadConfiguration(ctx context.Context, cfgChan chan<- json.Marshaler) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Initial configuration
	if err := p.updateConfiguration(ctx, cfgChan); err != nil {
		log.Printf("Error during initial configuration: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := p.updateConfiguration(ctx, cfgChan); err != nil {
				log.Printf("Error updating configuration: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *Provider) updateConfiguration(ctx context.Context, cfgChan chan<- json.Marshaler) error {
	servicesMap, err := getServiceMap(p.client, ctx)
	if err != nil {
		return fmt.Errorf("error getting service map: %w", err)
	}

	debug := p.client.LogLevel == internal.LogLevelDebug
	configuration := generateConfiguration(servicesMap, debug)
	cfgChan <- &dynamic.JSONPayload{Configuration: configuration}
	return nil
}

// Stop to stop the provider and the related go routines.
func (p *Provider) Stop() error {
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

// ParserConfig represents the configuration for the Proxmox API client
type ParserConfig struct {
	ApiEndpoint string
	TokenId     string
	Token       string
	LogLevel    string
	ValidateSSL bool
}

func newParserConfig(apiEndpoint, tokenID, token string, logLevel string, validateSSL bool) (ParserConfig, error) {
	if apiEndpoint == "" || tokenID == "" || token == "" {
		return ParserConfig{}, errors.New("missing mandatory values: apiEndpoint, tokenID or token")
	}
	return ParserConfig{
		ApiEndpoint: apiEndpoint,
		TokenId:     tokenID,
		Token:       token,
		LogLevel:    logLevel,
		ValidateSSL: validateSSL,
	}, nil
}

func newClient(pc ParserConfig) *internal.ProxmoxClient {
	return internal.NewProxmoxClient(pc.ApiEndpoint, pc.TokenId, pc.Token, pc.ValidateSSL, pc.LogLevel)
}

func logVersion(client *internal.ProxmoxClient, ctx context.Context) error {
	version, err := client.GetVersion(ctx)
	if err != nil {
		return err
	}
	log.Printf("Connected to Proxmox VE version %s", version.Release)
	return nil
}

func getServiceMap(client *internal.ProxmoxClient, ctx context.Context) (map[string][]internal.Service, error) {
	servicesMap := make(map[string][]internal.Service)

	nodes, err := client.GetNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("error scanning nodes: %w", err)
	}

	for _, nodeStatus := range nodes {
		// Skip nodes that Proxmox reports as not online. Empty status falls
		// through (treated as online) to preserve historic behavior on older
		// Proxmox versions that may not return the field.
		if nodeStatus.Status != "" && nodeStatus.Status != "online" {
			log.Printf("Skipping node %s: status=%s", nodeStatus.Node, nodeStatus.Status)
			continue
		}
		services, err := scanServices(client, ctx, nodeStatus.Node)
		if err != nil {
			log.Printf("Error scanning services on node %s: %v", nodeStatus.Node, err)
			continue
		}
		servicesMap[nodeStatus.Node] = services
	}
	return servicesMap, nil
}

// forbiddenWarnedVMIDs records VMIDs for which we've already emitted the
// "PVE 9 token role" hint, so the warning fires once per VM per process
// instead of on every poll.
var forbiddenWarnedVMIDs sync.Map

// maybeWarnForbidden emits a one-shot upgrade hint when the Proxmox API
// returned 403 for a guest's network-interface lookup. The most common cause
// in the field is a Proxmox VE 8 → 9 upgrade: PVE 9 removed the `VM.Monitor`
// privilege and split it into granular ones, so existing API tokens lose
// access to the QEMU guest-agent endpoint until `VM.GuestAgent.Audit` is
// added to their role.
func maybeWarnForbidden(err error, vmID uint64, isContainer bool) {
	if !errors.Is(err, internal.ErrForbidden) {
		return
	}
	if _, loaded := forbiddenWarnedVMIDs.LoadOrStore(vmID, struct{}{}); loaded {
		return
	}
	kind := "VM"
	if isContainer {
		kind = "container"
	}
	log.Printf(
		"WARN: 403 Forbidden from Proxmox network-interface API for %s %d. "+
			"If you upgraded to Proxmox VE 9, your API token role likely needs `VM.GuestAgent.Audit` "+
			"(VMs) and `VM.Audit` (containers). See README \"Proxmox API Token Setup\".",
		kind, vmID,
	)
}

func getIPsOfService(client *internal.ProxmoxClient, ctx context.Context, nodeName string, vmID uint64, isContainer bool) (ips []internal.IP, err error) {
	var agentInterfaces *internal.ParsedAgentInterfaces
	if isContainer {
		agentInterfaces, err = client.GetContainerNetworkInterfaces(ctx, nodeName, vmID)
		if err != nil {
			maybeWarnForbidden(err, vmID, true)
			log.Printf("ERROR: Error getting container network interfaces for %s/%d: %v", nodeName, vmID, err)
			return nil, fmt.Errorf("error getting container network interfaces: %w", err)
		}
	} else {
		agentInterfaces, err = client.GetVMNetworkInterfaces(ctx, nodeName, vmID)
		if err != nil {
			maybeWarnForbidden(err, vmID, false)
			log.Printf("ERROR: Error getting VM network interfaces for %s/%d: %v", nodeName, vmID, err)
			return nil, fmt.Errorf("error getting VM network interfaces: %w", err)
		}
	}

	rawIPs := agentInterfaces.GetIPs()

	filteredIPs := make([]internal.IP, 0)
	for _, ip := range rawIPs {
		if (ip.AddressType == "ipv4" || ip.AddressType == "inet") && ip.Address != "127.0.0.1" {
			filteredIPs = append(filteredIPs, ip)
		}
	}

	if len(filteredIPs) == 0 && client.LogLevel == internal.LogLevelDebug {
		log.Printf("ERROR: No valid IPs found for %s/%d (isContainer: %t). Raw IPs were: %+v", nodeName, vmID, isContainer, rawIPs)
	}

	return filteredIPs, nil
}

func scanServices(client *internal.ProxmoxClient, ctx context.Context, nodeName string) (services []internal.Service, err error) {
	// Scan virtual machines
	vms, err := client.GetVirtualMachines(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("error scanning VMs on node %s: %w", nodeName, err)
	}

	for _, vm := range vms {
		if client.LogLevel == internal.LogLevelDebug {
			log.Printf("DEBUG: Scanning VM %s/%s (%d): %s", nodeName, vm.Name, vm.VMID, vm.Status)
		}
		
		if vm.Status == "running" {
			config, err := client.GetVMConfig(ctx, nodeName, vm.VMID)
			if err != nil {
				log.Printf("ERROR: Error getting VM config for %d: %v", vm.VMID, err)
				continue
			}
			
			traefikConfig := config.GetTraefikMap()
			if client.LogLevel == internal.LogLevelDebug {
				log.Printf("VM %s (%d) traefik config: %v", vm.Name, vm.VMID, traefikConfig)
			}
			
			service := internal.NewService(vm.VMID, vm.Name, traefikConfig)
			
			ips, err := getIPsOfService(client, ctx, nodeName, vm.VMID, false)
			if err == nil {
				service.IPs = ips
			}

			services = append(services, service)
		}
	}

	// Scan containers
	cts, err := client.GetContainers(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("error scanning containers on node %s: %w", nodeName, err)
	}

	for _, ct := range cts {
		if client.LogLevel == internal.LogLevelDebug {
			log.Printf("DEBUG: Scanning container %s/%s (%d): %s", nodeName, ct.Name, ct.VMID, ct.Status)
		}
			

		if ct.Status == "running" {
			config, err := client.GetContainerConfig(ctx, nodeName, ct.VMID)
			if err != nil {
				log.Printf("ERROR: Error getting container config for %d: %v", ct.VMID, err)
				continue
			}

			traefikConfig := config.GetTraefikMap()
			if client.LogLevel == internal.LogLevelDebug {
				log.Printf("DEBUG: Container %s (%d) traefik config: %v", ct.Name, ct.VMID, traefikConfig)
			}

			service := internal.NewService(ct.VMID, ct.Name, traefikConfig)

			// Try to get container IPs if possible
			ips, err := getIPsOfService(client, ctx, nodeName, ct.VMID, true)
			if err == nil {
				service.IPs = ips
			}

			services = append(services, service)
		}
	}

	return services, nil
}

// generateConfiguration translates the per-node service map into a Traefik
// dynamic configuration. The `debug` flag controls verbose per-cycle logging
// (skipped/created services, IP fallbacks). Errors and warnings are emitted
// regardless of debug.
func generateConfiguration(servicesMap map[string][]internal.Service, debug bool) *dynamic.Configuration {
	config := &dynamic.Configuration{
		HTTP: &dynamic.HTTPConfiguration{
			Routers:           make(map[string]*dynamic.Router),
			Middlewares:       make(map[string]*dynamic.Middleware),
			Services:          make(map[string]*dynamic.Service),
			ServersTransports: make(map[string]*dynamic.ServersTransport),
		},
		TCP: &dynamic.TCPConfiguration{
			Routers:  make(map[string]*dynamic.TCPRouter),
			Services: make(map[string]*dynamic.TCPService),
		},
		UDP: &dynamic.UDPConfiguration{
			Routers:  make(map[string]*dynamic.UDPRouter),
			Services: make(map[string]*dynamic.UDPService),
		},
		TLS: &dynamic.TLSConfiguration{
			Stores:  make(map[string]tls.Store),
			Options: make(map[string]tls.Options),
		},
	}

	// Loop through all node service maps
	for nodeName, services := range servicesMap {
		// Loop through all services in this node
		for _, service := range services {
			// Skip disabled services
			if len(service.Config) == 0 || !isBoolLabelEnabled(service.Config, "traefik.enable") {
				if debug {
					log.Printf("Skipping service %s (ID: %d) because traefik.enable is not true", service.Name, service.ID)
				}
				continue
			}
			
			// Extract router and service names from labels. Names that contain
			// dots are rejected (warn-and-skip) because we cannot disambiguate
			// `routers.my.app.rule` ("router my, sub-key app.rule") from
			// ("router my.app, sub-key rule") without a complete sub-key
			// registry.
			routerPrefixMap := make(map[string]bool)
			servicePrefixMap := make(map[string]bool)

			for k := range service.Config {
				if strings.HasPrefix(k, "traefik.http.routers.") {
					name, ok := extractLabelName(k, "traefik.http.routers.", routerSubkeyAllowList)
					if !ok {
						log.Printf("Ignoring label %q on %s (ID %d): router name must be a single token without dots", k, service.Name, service.ID)
						continue
					}
					routerPrefixMap[name] = true
				}
				if strings.HasPrefix(k, "traefik.http.services.") {
					name, ok := extractLabelName(k, "traefik.http.services.", serviceSubkeyAllowList)
					if !ok {
						log.Printf("Ignoring label %q on %s (ID %d): service name must be a single token without dots", k, service.Name, service.ID)
						continue
					}
					servicePrefixMap[name] = true
				}
			}
			
			// Default to "<name>-<vmid>" if no explicit names found in labels.
			defaultID := fmt.Sprintf("%s-%d", service.Name, service.ID)

			// Convert maps to (sorted) slices
			routerNames := mapKeysToSlice(routerPrefixMap)
			serviceNames := mapKeysToSlice(servicePrefixMap)

			// Use defaults if no names found
			if len(routerNames) == 0 {
				routerNames = []string{defaultID}
			}
			if len(serviceNames) == 0 {
				serviceNames = []string{defaultID}
			}

			// Namespacing: VMID is cluster-wide unique in Proxmox, so suffixing
			// user-supplied router/service names with "-<vmid>" makes them
			// globally unique and prevents silent overwrites when multiple
			// guests share the same router/service name (e.g. "app").
			//
			// The defaultID branch already includes the VMID, so we leave
			// it untouched to avoid producing keys like "web-101-101".
			ns := func(name string) string {
				if name == defaultID {
					return name
				}
				return fmt.Sprintf("%s-%d", name, service.ID)
			}

			// Build a set of services declared on THIS guest. Used below to
			// rewrite same-guest router→service cross-references; references
			// to services not in this set are passed through verbatim, since
			// they may point at the file/static config (e.g. "external@file").
			localServices := make(map[string]bool, len(serviceNames))
			for _, s := range serviceNames {
				localServices[s] = true
			}

			// Create services
			for _, serviceName := range serviceNames {
				// Configure load balancer options
				loadBalancer := &dynamic.ServersLoadBalancer{
					PassHostHeader: boolPtr(true), // Default is true
					Servers:        []dynamic.Server{},
				}

				// Apply service options. Note: helpers below take the ORIGINAL
				// (un-namespaced) name because they look up labels in
				// service.Config, which is keyed by the original.
				applyServiceOptions(loadBalancer, service, serviceName)

				// Add one Server entry per discovered backend URL.
				for _, u := range getServiceURLs(service, serviceName, nodeName, debug) {
					loadBalancer.Servers = append(loadBalancer.Servers, dynamic.Server{URL: u})
				}

				config.HTTP.Services[ns(serviceName)] = &dynamic.Service{
					LoadBalancer: loadBalancer,
				}
			}

			// Create routers
			for _, routerName := range routerNames {
				// Get router rule (original name — reads labels)
				rule := getRouterRule(service, routerName)

				// Find target service. Prefer the explicit `…service=<svc>`
				// mapping; if it points at a service declared on this same
				// guest, rewrite it to the namespaced form so the link still
				// resolves after we suffix names with -<vmid>. Otherwise the
				// reference is foreign (e.g. "external@file") and stays as-is.
				targetService := ns(serviceNames[0])
				serviceLabel := fmt.Sprintf("traefik.http.routers.%s.service", routerName)
				if val, exists := service.Config[serviceLabel]; exists {
					if localServices[val] {
						targetService = ns(val)
					} else {
						targetService = val
					}
				}

				// Create basic router. Leave Priority unset (zero) so Traefik
				// applies its default rule-length-based ordering. The
				// applyRouterOptions call below will set Priority if the user
				// provides an explicit `…priority=N` label.
				router := &dynamic.Router{
					Service: targetService,
					Rule:    rule,
				}

				// Apply additional router options from labels (original name).
				applyRouterOptions(router, service, routerName)

				config.HTTP.Routers[ns(routerName)] = router
			}

			if debug {
				log.Printf("Created router and service for %s (ID: %d)", service.Name, service.ID)
			}
		}
	}
	
	return config
}

// Apply router configuration options from labels
func applyRouterOptions(router *dynamic.Router, service internal.Service, routerName string) {
	prefix := fmt.Sprintf("traefik.http.routers.%s", routerName)
	
	// Handle EntryPoints
	if entrypoints, exists := service.Config[prefix+".entrypoints"]; exists {
		// Backward compatibility with singular form
		router.EntryPoints = strings.Split(entrypoints, ",")
	} else if entrypoint, exists := service.Config[prefix+".entrypoint"]; exists {
		router.EntryPoints = []string{entrypoint}
	}
	
	// Handle Middlewares
	if middlewares, exists := service.Config[prefix+".middlewares"]; exists {
		router.Middlewares = strings.Split(middlewares, ",")
	}
	
	// Handle Priority
	if priority, exists := service.Config[prefix+".priority"]; exists {
		if p, err := stringToInt(priority); err == nil {
			router.Priority = p
		}
	}
	
	// Handle TLS
	tls := handleRouterTLS(service, prefix)
	if tls != nil {
		router.TLS = tls
	}
}

// Apply service configuration options from labels
func applyServiceOptions(lb *dynamic.ServersLoadBalancer, service internal.Service, serviceName string) {
	prefix := fmt.Sprintf("traefik.http.services.%s.loadbalancer", serviceName)
	
	// Handle PassHostHeader
	if passHostHeader, exists := service.Config[prefix+".passhostheader"]; exists {
		if val, err := stringToBool(passHostHeader); err == nil {
			lb.PassHostHeader = &val
		}
	}
	
	// Handle HealthCheck
	if healthcheckPath, exists := service.Config[prefix+".healthcheck.path"]; exists {
		hc := &dynamic.ServerHealthCheck{
			Path: healthcheckPath,
		}
		
		if interval, exists := service.Config[prefix+".healthcheck.interval"]; exists {
			hc.Interval = interval
		}
		
		if timeout, exists := service.Config[prefix+".healthcheck.timeout"]; exists {
			hc.Timeout = timeout
		}
		
		lb.HealthCheck = hc
	}
	
	// Handle Sticky Sessions
	if cookieName, exists := service.Config[prefix+".sticky.cookie.name"]; exists {
		sticky := &dynamic.Sticky{
			Cookie: &dynamic.Cookie{
				Name: cookieName,
			},
		}
		
		if secure, exists := service.Config[prefix+".sticky.cookie.secure"]; exists {
			if val, err := stringToBool(secure); err == nil {
				sticky.Cookie.Secure = val
			}
		}
		
		if httpOnly, exists := service.Config[prefix+".sticky.cookie.httponly"]; exists {
			if val, err := stringToBool(httpOnly); err == nil {
				sticky.Cookie.HTTPOnly = val
			}
		}
		
		lb.Sticky = sticky
	}
	
	// Handle ResponseForwarding
	if flushInterval, exists := service.Config[prefix+".responseforwarding.flushinterval"]; exists {
		lb.ResponseForwarding = &dynamic.ResponseForwarding{
			FlushInterval: flushInterval,
		}
	}
	
	// Handle ServerTransport
	if serverTransport, exists := service.Config[prefix+".serverstransport"]; exists {
		lb.ServersTransport = serverTransport
	}
}

// tlsDomainPattern matches array-indexed TLS domain labels of the form
// `…tls.domains[N].main` or `…tls.domains[N].sans`. Compiled once at package
// init to avoid recompiling on every poll cycle.
var tlsDomainPattern = regexp.MustCompile(`\.tls\.domains\[(\d+)\]\.(main|sans)$`)

// routerSubkeyAllowList is the set of first-segment subkeys that may follow a
// router name in a `traefik.http.routers.<name>.<subkey…>` label. Used by
// extractLabelName to detect when a user has accidentally put dots inside the
// router name itself (which would be ambiguous to parse). Keep this in sync
// with applyRouterOptions / handleRouterTLS.
var routerSubkeyAllowList = map[string]bool{
	"rule":        true,
	"entrypoints": true,
	"entrypoint":  true, // backward-compat singular form
	"middlewares": true,
	"priority":    true,
	"service":     true,
	"tls":         true,
}

// serviceSubkeyAllowList is the equivalent for `traefik.http.services.<name>.<subkey…>`.
// Today only `loadbalancer` exists as a top-level subkey under a service.
var serviceSubkeyAllowList = map[string]bool{
	"loadbalancer": true,
}

// extractLabelName parses a label key of the form `<prefix><name>.<subkey…>`
// and returns the name. It returns ok=false when:
//   - the key does not start with the prefix
//   - the key has no subkey portion at all
//   - the first dot-segment of the subkey is not in `allow` (which strongly
//     suggests the user put dots in the name itself; we cannot disambiguate
//     "router named my.app with sub-key rule" from "router named my with
//     sub-key app.rule" without a complete subkey registry)
//
// Callers that get ok=false should log a warning and skip the label.
func extractLabelName(key, prefix string, allow map[string]bool) (string, bool) {
	rest := strings.TrimPrefix(key, prefix)
	if rest == key {
		return "", false
	}
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) < 2 || parts[0] == "" {
		return "", false
	}
	name, subkey := parts[0], parts[1]
	// First dot-segment of the subkey (e.g. "tls" from "tls.options").
	firstSub := subkey
	if i := strings.Index(firstSub, "."); i >= 0 {
		firstSub = firstSub[:i]
	}
	// Strip array indexer like `domains[0]` so we compare the bare token.
	if i := strings.Index(firstSub, "["); i >= 0 {
		firstSub = firstSub[:i]
	}
	if !allow[firstSub] {
		return "", false
	}
	return name, true
}

// Handle TLS configuration
func handleRouterTLS(service internal.Service, prefix string) *dynamic.RouterTLSConfig {
	tlsEnabled := false
	if tlsLabel, exists := service.Config[prefix+".tls"]; exists {
		if tlsLabel == "true" {
			tlsEnabled = true
		}
	}

	certResolver, hasCertResolver := service.Config[prefix+".tls.certresolver"]
	domains, hasDomains := service.Config[prefix+".tls.domains"]
	options, hasOptions := service.Config[prefix+".tls.options"]

	// Check for array-indexed domains: tls.domains[N].main/sans
	domainMap := make(map[int]*types.Domain)
	for key, value := range service.Config {
		if matches := tlsDomainPattern.FindStringSubmatch(key); matches != nil {
			idx, _ := strconv.Atoi(matches[1])
			if domainMap[idx] == nil {
				domainMap[idx] = &types.Domain{}
			}
			if matches[2] == "main" {
				domainMap[idx].Main = value
			} else {
				domainMap[idx].SANs = strings.Split(value, ",")
			}
		}
	}
	hasArrayDomains := len(domainMap) > 0

	if !tlsEnabled && !hasCertResolver && !hasDomains && !hasOptions && !hasArrayDomains {
		return nil
	}

	tlsConfig := &dynamic.RouterTLSConfig{}

	if hasCertResolver {
		tlsConfig.CertResolver = certResolver
	}

	if hasOptions {
		tlsConfig.Options = options
	}

	// Array-indexed domains take precedence
	if hasArrayDomains {
		indices := make([]int, 0, len(domainMap))
		for idx := range domainMap {
			indices = append(indices, idx)
		}
		sort.Ints(indices)
		for _, idx := range indices {
			tlsConfig.Domains = append(tlsConfig.Domains, *domainMap[idx])
		}
	} else if hasDomains {
		for _, domain := range strings.Split(domains, ",") {
			tlsConfig.Domains = append(tlsConfig.Domains, types.Domain{Main: domain})
		}
	}

	return tlsConfig
}

// getServiceURLs builds the list of backend URLs for a service. Each entry
// becomes one Server in the load balancer. Precedence (first match wins,
// returning a single-element slice):
//  1. Explicit `…server.url` label
//  2. Explicit `…server.ip` label (combined with port + scheme)
//  3. Discovered guest-agent IPs (one URL per IP, sorted by address for
//     deterministic output across polls)
//  4. Hostname fallback `<vmname>.<nodename>:<port>` (logged at debug only;
//     usually means the guest agent isn't reporting IPs)
func getServiceURLs(service internal.Service, serviceName string, nodeName string, debug bool) []string {
	// Check for direct URL override
	urlLabel := fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.url", serviceName)
	if url, exists := service.Config[urlLabel]; exists {
		return []string{url}
	}

	// Default protocol and port
	protocol := "http"
	port := "80"

	// Check for HTTPS protocol setting
	httpsLabel := fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.scheme", serviceName)
	if scheme, exists := service.Config[httpsLabel]; exists && scheme == "https" {
		protocol = "https"
		// Update default port for HTTPS
		port = "443"
	}

	// Look for service-specific port
	portLabel := fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", serviceName)
	if val, exists := service.Config[portLabel]; exists {
		port = val
	}

	// Look for service-specific ip
	ipLabel := fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.ip", serviceName)
	if val, exists := service.Config[ipLabel]; exists {
		return []string{fmt.Sprintf("%s://%s:%s", protocol, val, port)}
	}

	// Use guest-agent IPs if available, otherwise fall back to hostname.
	if len(service.IPs) > 0 {
		urls := make([]string, 0, len(service.IPs))
		for _, ip := range service.IPs {
			if ip.Address != "" {
				urls = append(urls, fmt.Sprintf("%s://%s:%s", protocol, ip.Address, port))
			}
		}
		if len(urls) > 0 {
			// Sort so the emitted load-balancer config is stable across polls
			// even if the guest agent reorders interfaces.
			sort.Strings(urls)
			return urls
		}
	}

	// Fall back to hostname
	url := fmt.Sprintf("%s://%s.%s:%s", protocol, service.Name, nodeName, port)
	if debug {
		log.Printf("No IPs found, using hostname URL %s for service %s (ID: %d)", url, service.Name, service.ID)
	}
	return []string{url}
}

// Helper to get router rule
func getRouterRule(service internal.Service, routerName string) string {
	// Default rule
	rule := fmt.Sprintf("Host(`%s`)", service.Name)
	
	// Look for router-specific rule
	ruleLabel := fmt.Sprintf("traefik.http.routers.%s.rule", routerName)
	if val, exists := service.Config[ruleLabel]; exists {
		rule = val
	}
	
	return rule
}

// Helper to convert string to int
func stringToInt(s string) (int, error) {
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err != nil {
		return 0, err
	}
	return i, nil
}

// Helper to convert string to bool
func stringToBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("cannot convert %s to bool", s)
	}
}

// mapKeysToSlice returns the keys of m as a slice in deterministic
// (lexicographic) order. Sorting matters because callers iterate the result
// to build router/service configuration; without it, downstream behavior
// (e.g. which service a router defaults to) flaps between polls.
func mapKeysToSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

func boolPtr(v bool) *bool {
	return &v
}

// validateConfig validates the plugin configuration
func validateConfig(config *Config) error {
	if config == nil {
		return errors.New("configuration cannot be nil")
	}

	if config.PollInterval == "" {
		return errors.New("poll interval must be set")
	}

	if config.ApiEndpoint == "" {
		return errors.New("API endpoint must be set")
	}

	if config.ApiTokenId == "" {
		return errors.New("API token ID must be set")
	}

	if config.ApiToken == "" {
		return errors.New("API token must be set")
	}

	return nil
}

// isBoolLabelEnabled returns true when `label` is present in `labels` and its
// value parses as a Go bool via strconv.ParseBool. This matches Traefik's own
// Docker provider semantics — accepts "1", "t", "T", "true", "True", "TRUE"
// (and "false", "0", "f" etc. as false). It intentionally does NOT use the
// more permissive stringToBool helper, which accepts "yes"/"on"/"off" — those
// are not recognized by Traefik core.
func isBoolLabelEnabled(labels map[string]string, label string) bool {
	val, exists := labels[label]
	if !exists {
		return false
	}
	b, err := strconv.ParseBool(val)
	return err == nil && b
}
