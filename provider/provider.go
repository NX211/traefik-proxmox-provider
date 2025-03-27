// Package provider is a plugin to use a proxmox cluster as an provider.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/NX211/traefik-proxmox-provider/internal"
	"github.com/traefik/genconf/dynamic"
	"github.com/traefik/genconf/dynamic/tls"
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
	)
	if err != nil {
		return nil, fmt.Errorf("invalid parser config: %w", err)
	}

	pc.LogLevel = config.ApiLogging
	pc.ValidateSSL = config.ApiValidateSSL == "true"
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

	configuration := generateConfiguration(time.Now(), servicesMap)
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

func newParserConfig(apiEndpoint, tokenID, token string) (ParserConfig, error) {
	if apiEndpoint == "" || tokenID == "" || token == "" {
		return ParserConfig{}, errors.New("missing mandatory values: apiEndpoint, tokenID or token")
	}
	return ParserConfig{
		ApiEndpoint: apiEndpoint,
		TokenId:     tokenID,
		Token:       token,
		LogLevel:    "info",
		ValidateSSL: true,
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
		services, err := scanServices(client, ctx, nodeStatus.Node)
		if err != nil {
			log.Printf("Error scanning services on node %s: %v", nodeStatus.Node, err)
			continue
		}
		servicesMap[nodeStatus.Node] = services
	}
	return servicesMap, nil
}

func getIPsOfService(client *internal.ProxmoxClient, ctx context.Context, nodeName string, vmID uint64) (ips []internal.IP, err error) {
	interfaces, err := client.GetVMNetworkInterfaces(ctx, nodeName, vmID)
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}
	return interfaces.GetIPs(), nil
}

func scanServices(client *internal.ProxmoxClient, ctx context.Context, nodeName string) (services []internal.Service, err error) {
	// Scan virtual machines
	vms, err := client.GetVirtualMachines(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("error scanning VMs on node %s: %w", nodeName, err)
	}

	for _, vm := range vms {
		log.Printf("Scanning VM %s/%s (%d): %s", nodeName, vm.Name, vm.VMID, vm.Status)
		
		if vm.Status == "running" {
			config, err := client.GetVMConfig(ctx, nodeName, vm.VMID)
			if err != nil {
				log.Printf("Error getting VM config for %d: %v", vm.VMID, err)
				continue
			}
			
			traefikConfig := config.GetTraefikMap()
			log.Printf("VM %s (%d) traefik config: %v", vm.Name, vm.VMID, traefikConfig)
			
			service := internal.NewService(vm.VMID, vm.Name, traefikConfig)
			
			ips, err := getIPsOfService(client, ctx, nodeName, vm.VMID)
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
		log.Printf("Scanning container %s/%s (%d): %s", nodeName, ct.Name, ct.VMID, ct.Status)
		
		if ct.Status == "running" {
			config, err := client.GetContainerConfig(ctx, nodeName, ct.VMID)
			if err != nil {
				log.Printf("Error getting container config for %d: %v", ct.VMID, err)
				continue
			}
			
			traefikConfig := config.GetTraefikMap()
			log.Printf("Container %s (%d) traefik config: %v", ct.Name, ct.VMID, traefikConfig)
			
			service := internal.NewService(ct.VMID, ct.Name, traefikConfig)
			
			// Try to get container IPs if possible
			ips, err := getIPsOfService(client, ctx, nodeName, ct.VMID)
			if err == nil {
				service.IPs = ips
			}
			
			services = append(services, service)
		}
	}

	return services, nil
}

func generateConfiguration(date time.Time, servicesMap map[string][]internal.Service) *dynamic.Configuration {
	configuration := &dynamic.Configuration{
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
		TLS: &dynamic.TLSConfiguration{
			Stores:  make(map[string]tls.Store),
			Options: make(map[string]tls.Options),
		},
		UDP: &dynamic.UDPConfiguration{
			Routers:  make(map[string]*dynamic.UDPRouter),
			Services: make(map[string]*dynamic.UDPService),
		},
	}

	// Loop through all node service maps
	for nodeName, services := range servicesMap {
		// Loop through all services in this node
		for _, service := range services {
			// Check if traefik.enable is set to true
			if enable, exists := service.Config["traefik.enable"]; !exists || enable != "true" {
				log.Printf("Skipping service %s (ID: %d) because traefik.enable is not true", service.Name, service.ID)
				continue
			}

			// Service name will be used to identify this service
			serviceName := fmt.Sprintf("%s-%d", service.Name, service.ID)
			
			// Create a default LoadBalancer service
			lb := &dynamic.ServersLoadBalancer{
				PassHostHeader: boolPtr(true),
				Servers:        []dynamic.Server{},
			}
			
			// Add server endpoints based on IPs
			if len(service.IPs) > 0 {
				log.Printf("Found %d IPs for service %s (ID: %d)", len(service.IPs), service.Name, service.ID)
				for _, ip := range service.IPs {
					if ip.Address != "" {
						// Default to port 80 if not specified
						port := "80"
						if customPort, exists := service.Config["traefik.http.services.port"]; exists {
							port = customPort
						}
						url := fmt.Sprintf("http://%s:%s", ip.Address, port)
						lb.Servers = append(lb.Servers, dynamic.Server{URL: url})
						log.Printf("Added server URL %s for service %s (ID: %d)", url, service.Name, service.ID)
					}
				}
			} else {
				// If no IPs found, try to use VM/container name as hostname
				port := "80"
				if customPort, exists := service.Config["traefik.http.services.port"]; exists {
					port = customPort
				}
				url := fmt.Sprintf("http://%s.%s:%s", service.Name, nodeName, port)
				lb.Servers = append(lb.Servers, dynamic.Server{URL: url})
				log.Printf("No IPs found, using hostname URL %s for service %s (ID: %d)", url, service.Name, service.ID)
			}
			
			// Create the service if we have servers
			if len(lb.Servers) > 0 {
				configuration.HTTP.Services[serviceName] = &dynamic.Service{
					LoadBalancer: lb,
				}
				
				// Default router rule
				routerRule := fmt.Sprintf("Host(`%s`)", service.Name)
				
				// Check for custom router rule
				if customRule, exists := service.Config["traefik.http.routers.rule"]; exists {
					routerRule = customRule
				}
				
				// Create the router
				configuration.HTTP.Routers[serviceName] = &dynamic.Router{
					Service:  serviceName,
					Rule:     routerRule,
					Priority: 1,
				}
				
				log.Printf("Created router and service for %s (ID: %d) with rule %s", service.Name, service.ID, routerRule)
			}
		}
	}

	return configuration
}

func boolPtr(v bool) *bool {
	return &v
}

func intPtr(v int) *int {
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
