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
	"time"

	"github.com/NX211/traefik-proxmox-provider/internal"
	"github.com/traefik/genconf/dynamic"
	"github.com/traefik/genconf/dynamic/tls"
	"github.com/traefik/genconf/dynamic/types"
)

// NodeConfig represents the configuration for a single Proxmox node/cluster endpoint.
type NodeConfig struct {
	Name           string `json:"name" yaml:"name" toml:"name"`
	ApiEndpoint    string `json:"apiEndpoint" yaml:"apiEndpoint" toml:"apiEndpoint"`
	ApiTokenId     string `json:"apiTokenId" yaml:"apiTokenId" toml:"apiTokenId"`
	ApiToken       string `json:"apiToken" yaml:"apiToken" toml:"apiToken"`
	ApiLogging     string `json:"apiLogging" yaml:"apiLogging" toml:"apiLogging"`
	ApiValidateSSL string `json:"apiValidateSSL" yaml:"apiValidateSSL" toml:"apiValidateSSL"`
}

// Config the plugin configuration.
type Config struct {
	PollInterval   string       `json:"pollInterval" yaml:"pollInterval" toml:"pollInterval"`
	ApiEndpoint    string       `json:"apiEndpoint" yaml:"apiEndpoint" toml:"apiEndpoint"`
	ApiTokenId     string       `json:"apiTokenId" yaml:"apiTokenId" toml:"apiTokenId"`
	ApiToken       string       `json:"apiToken" yaml:"apiToken" toml:"apiToken"`
	ApiLogging     string       `json:"apiLogging" yaml:"apiLogging" toml:"apiLogging"`
	ApiValidateSSL string       `json:"apiValidateSSL" yaml:"apiValidateSSL" toml:"apiValidateSSL"`
	Nodes          []NodeConfig `json:"nodes" yaml:"nodes" toml:"nodes"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		PollInterval:   "30s", // Default to 30 seconds for polling
		ApiValidateSSL: "true",
		ApiLogging:     "info",
	}
}

// namedClient associates a logical name with a Proxmox API client.
type namedClient struct {
	name   string
	client *internal.ProxmoxClient
}

// Provider a plugin.
type Provider struct {
	name         string
	pollInterval time.Duration
	clients      []*namedClient
	cancel       func()
}

// normalizeNodes converts the Config into a canonical []NodeConfig slice.
// If Nodes is populated it is returned directly; otherwise a single-element
// slice is synthesized from the legacy flat fields for backward compatibility.
func normalizeNodes(config *Config) []NodeConfig {
	if len(config.Nodes) > 0 {
		// Fill in defaults for nodes that omit optional fields.
		for i := range config.Nodes {
			if config.Nodes[i].ApiLogging == "" {
				config.Nodes[i].ApiLogging = "info"
			}
			if config.Nodes[i].ApiValidateSSL == "" {
				config.Nodes[i].ApiValidateSSL = "true"
			}
			if config.Nodes[i].Name == "" {
				config.Nodes[i].Name = fmt.Sprintf("node-%d", i)
			}
		}
		return config.Nodes
	}

	return []NodeConfig{
		{
			Name:           "default",
			ApiEndpoint:    config.ApiEndpoint,
			ApiTokenId:     config.ApiTokenId,
			ApiToken:       config.ApiToken,
			ApiLogging:     config.ApiLogging,
			ApiValidateSSL: config.ApiValidateSSL,
		},
	}
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

	nodes := normalizeNodes(config)
	var clients []*namedClient

	for _, node := range nodes {
		pc, err := newParserConfig(
			node.ApiEndpoint,
			node.ApiTokenId,
			node.ApiToken,
			node.ApiLogging,
			node.ApiValidateSSL == "true",
		)
		if err != nil {
			return nil, fmt.Errorf("invalid parser config for node %q: %w", node.Name, err)
		}
		client := newClient(pc)

		if err := logVersion(client, ctx); err != nil {
			return nil, fmt.Errorf("failed to get Proxmox version for node %q: %w", node.Name, err)
		}

		clients = append(clients, &namedClient{name: node.Name, client: client})
	}

	return &Provider{
		name:         name,
		pollInterval: pi,
		clients:      clients,
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
	servicesMap, err := getServiceMaps(p.clients, ctx)
	if err != nil {
		return fmt.Errorf("error getting service map: %w", err)
	}

	multiCluster := len(p.clients) > 1
	configuration := generateConfiguration(servicesMap, multiCluster)
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

// clusterServiceMap holds the services discovered from a single Proxmox endpoint,
// keyed by Proxmox node name, along with the logical name of the endpoint.
type clusterServiceMap struct {
	clusterName string
	services    map[string][]internal.Service
}

func getServiceMaps(clients []*namedClient, ctx context.Context) ([]clusterServiceMap, error) {
	var results []clusterServiceMap

	for _, nc := range clients {
		servicesMap := make(map[string][]internal.Service)

		nodes, err := nc.client.GetNodes(ctx)
		if err != nil {
			log.Printf("Error scanning nodes for %q: %v", nc.name, err)
			continue
		}

		for _, nodeStatus := range nodes {
			services, err := scanServices(nc.client, ctx, nodeStatus.Node)
			if err != nil {
				log.Printf("Error scanning services on node %s (%s): %v", nodeStatus.Node, nc.name, err)
				continue
			}
			servicesMap[nodeStatus.Node] = services
		}

		results = append(results, clusterServiceMap{
			clusterName: nc.name,
			services:    servicesMap,
		})
	}

	return results, nil
}

func getIPsOfService(client *internal.ProxmoxClient, ctx context.Context, nodeName string, vmID uint64, isContainer bool) (ips []internal.IP, err error) {
	var agentInterfaces *internal.ParsedAgentInterfaces
	if isContainer {
		agentInterfaces, err = client.GetContainerNetworkInterfaces(ctx, nodeName, vmID)
		if err != nil {
			log.Printf("ERROR: Error getting container network interfaces for %s/%d: %v", nodeName, vmID, err)
			return nil, fmt.Errorf("error getting container network interfaces: %w", err)
		}
	} else {
		agentInterfaces, err = client.GetVMNetworkInterfaces(ctx, nodeName, vmID)
		if err != nil {
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
		if client.LogLevel == "debug" {
			log.Printf("DEBUG: Scanning VM %s/%s (%d): %s", nodeName, vm.Name, vm.VMID, vm.Status)
		}
		
		if vm.Status == "running" {
			config, err := client.GetVMConfig(ctx, nodeName, vm.VMID)
			if err != nil {
				log.Printf("ERROR: Error getting VM config for %d: %v", vm.VMID, err)
				continue
			}
			
			traefikConfig := config.GetTraefikMap()
			if client.LogLevel == "debug" {
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
		if client.LogLevel == "debug" {
			log.Printf("DEBUG: Scanning container %s/%s (%d): %s", nodeName, ct.Name, ct.VMID, ct.Status)
		}
			

		if ct.Status == "running" {
			config, err := client.GetContainerConfig(ctx, nodeName, ct.VMID)
			if err != nil {
				log.Printf("ERROR: Error getting container config for %d: %v", ct.VMID, err)
				continue
			}

			traefikConfig := config.GetTraefikMap()
			if client.LogLevel == "debug" {
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

func generateConfiguration(clusterMaps []clusterServiceMap, multiCluster bool) *dynamic.Configuration {
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

	// Loop through all cluster service maps
	for _, csm := range clusterMaps {
		// Loop through all node service maps in this cluster
		for nodeName, services := range csm.services {
			// Loop through all services in this node
			for _, service := range services {
				// Skip disabled services
				if len(service.Config) == 0 || !isBoolLabelEnabled(service.Config, "traefik.enable") {
					log.Printf("Skipping service %s (ID: %d) because traefik.enable is not true", service.Name, service.ID)
					continue
				}

				// Extract router and service names from labels
				routerPrefixMap := make(map[string]bool)
				servicePrefixMap := make(map[string]bool)

				for k := range service.Config {
					if strings.HasPrefix(k, "traefik.http.routers.") {
						parts := strings.Split(k, ".")
						if len(parts) > 3 {
							routerPrefixMap[parts[3]] = true
						}
					}
					if strings.HasPrefix(k, "traefik.http.services.") {
						parts := strings.Split(k, ".")
						if len(parts) > 3 {
							servicePrefixMap[parts[3]] = true
						}
					}
				}

				// Default to service ID if no names found.
				// When multiple clusters are configured, prefix with the cluster
				// name to avoid collisions between independent Proxmox installations.
				var defaultID string
				if multiCluster {
					defaultID = fmt.Sprintf("%s-%s-%d", csm.clusterName, service.Name, service.ID)
				} else {
					defaultID = fmt.Sprintf("%s-%d", service.Name, service.ID)
				}

				// Convert maps to slices
				routerNames := mapKeysToSlice(routerPrefixMap)
				serviceNames := mapKeysToSlice(servicePrefixMap)

				// Use defaults if no names found
				if len(routerNames) == 0 {
					routerNames = []string{defaultID}
				}
				if len(serviceNames) == 0 {
					serviceNames = []string{defaultID}
				}

				// Create services
				for _, serviceName := range serviceNames {
					// Configure load balancer options
					loadBalancer := &dynamic.ServersLoadBalancer{
						PassHostHeader: boolPtr(true), // Default is true
						Servers:        []dynamic.Server{},
					}

					// Apply service options
					applyServiceOptions(loadBalancer, service, serviceName)

					// Add server URL(s)
					serverURL := getServiceURL(service, serviceName, nodeName)
					loadBalancer.Servers = append(loadBalancer.Servers, dynamic.Server{
						URL: serverURL,
					})

					config.HTTP.Services[serviceName] = &dynamic.Service{
						LoadBalancer: loadBalancer,
					}
				}

				// Create routers
				for _, routerName := range routerNames {
					// Get router rule
					rule := getRouterRule(service, routerName)

					// Find target service (prefer explicit mapping)
					targetService := serviceNames[0]
					serviceLabel := fmt.Sprintf("traefik.http.routers.%s.service", routerName)
					if val, exists := service.Config[serviceLabel]; exists {
						targetService = val
					}

					// Create basic router
					router := &dynamic.Router{
						Service:  targetService,
						Rule:     rule,
						Priority: 1, // Default priority
					}

					// Apply additional router options from labels
					applyRouterOptions(router, service, routerName)

					config.HTTP.Routers[routerName] = router
				}

				log.Printf("Created router and service for %s (ID: %d) on %s", service.Name, service.ID, csm.clusterName)
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
	domainPattern := regexp.MustCompile(`\.tls\.domains\[(\d+)\]\.(main|sans)$`)
	domainMap := make(map[int]*types.Domain)
	for key, value := range service.Config {
		if matches := domainPattern.FindStringSubmatch(key); matches != nil {
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

// Helper to get service URL with correct port
func getServiceURL(service internal.Service, serviceName string, nodeName string) string {
	// Check for direct URL override
	urlLabel := fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.url", serviceName)
	if url, exists := service.Config[urlLabel]; exists {
		return url
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
		return fmt.Sprintf("%s://%s:%s", protocol, val, port)
	}
	
	// Use IP if available, otherwise fall back to hostname
	if len(service.IPs) > 0 {
		// Create a list of server URLs from all IPs
		for _, ip := range service.IPs {
			if ip.Address != "" {
				return fmt.Sprintf("%s://%s:%s", protocol, ip.Address, port)
			}
		}
	}
	
	// Fall back to hostname
	url := fmt.Sprintf("%s://%s.%s:%s", protocol, service.Name, nodeName, port)
	log.Printf("No IPs found, using hostname URL %s for service %s (ID: %d)", url, service.Name, service.ID)
	return url
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

// Helper to convert map keys to slice
func mapKeysToSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func boolPtr(v bool) *bool {
	return &v
}

// validateNodeConfig validates a single node configuration entry.
func validateNodeConfig(nc NodeConfig, label string) error {
	if nc.ApiEndpoint == "" {
		return fmt.Errorf("%s: API endpoint must be set", label)
	}
	if nc.ApiTokenId == "" {
		return fmt.Errorf("%s: API token ID must be set", label)
	}
	if nc.ApiToken == "" {
		return fmt.Errorf("%s: API token must be set", label)
	}
	return nil
}

// validateConfig validates the plugin configuration
func validateConfig(config *Config) error {
	if config == nil {
		return errors.New("configuration cannot be nil")
	}

	if config.PollInterval == "" {
		return errors.New("poll interval must be set")
	}

	// Multi-node configuration
	if len(config.Nodes) > 0 {
		for i, node := range config.Nodes {
			label := fmt.Sprintf("nodes[%d]", i)
			if node.Name != "" {
				label = fmt.Sprintf("node %q", node.Name)
			}
			if err := validateNodeConfig(node, label); err != nil {
				return err
			}
		}
		return nil
	}

	// Legacy single-node configuration
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

func isBoolLabelEnabled(labels map[string]string, label string) bool {
	val, exists := labels[label]
	return exists && val == "true"
}
