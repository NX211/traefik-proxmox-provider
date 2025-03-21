// Package proxmox_plugin is a plugin to use a proxmox cluster as an provider.
package proxmox_plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/NX211/traefik-proxmox-provider/internal"
	"github.com/luthermonson/go-proxmox"
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
		PollInterval:   "5s", // 5 * time.Second
		ApiValidateSSL: "true",
		ApiLogging:     "info",
	}
}

// Provider a plugin.
type Provider struct {
	name         string
	pollInterval time.Duration
	client       *proxmox.Client
	cancel       func()
}

// New creates a new Provider plugin.
func New(ctx context.Context, config *Config, name string) (*Provider, error) {
	pi, err := time.ParseDuration(config.PollInterval)
	if err != nil {
		return nil, err
	}

	if len(config.PollInterval) <= 0 {
		return nil, fmt.Errorf("poll interval must be greater than 0")
	}

	if len(config.ApiEndpoint) <= 0 {
		return nil, fmt.Errorf("api Endpoint has to be set")
	}

	pc, err := NewParserConfig(
		config.ApiEndpoint,
		config.ApiTokenId,
		config.ApiToken,
	)
	if err != nil {
		return nil, err
	}

	pc.LogLevel = config.ApiLogging
	pc.ValidateSSL = config.ApiValidateSSL == "true"
	client := NewClient(pc)

	err = LogVersion(client, ctx)
	if err != nil {
		return nil, err
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
				log.Print(err)
			}
		}()

		p.loadConfiguration(ctx, cfgChan)
	}()

	return nil
}

func (p *Provider) loadConfiguration(ctx context.Context, cfgChan chan<- json.Marshaler) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case t := <-ticker.C:
			servicesMap, err := GetServiceMap(p.client, ctx)
			if err != nil {
				log.Print(err)
			} else {
				configuration := generateConfiguration(t, servicesMap)
				cfgChan <- &dynamic.JSONPayload{Configuration: configuration}
			}
		case <-ctx.Done():
			return
		}
	}
}

// Stop to stop the provider and the related go routines.
func (p *Provider) Stop() error {
	p.cancel()
	return nil
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

	// configuration.HTTP.Routers["pp-route-01"] = &dynamic.Router{
	// 	EntryPoints: []string{"web"},
	// 	Service:     "pp-service-01",
	// 	Rule:        "Host(`example.com`)",
	// }

	// configuration.HTTP.Services["pp-service-01"] = &dynamic.Service{
	// 	LoadBalancer: &dynamic.ServersLoadBalancer{
	// 		Servers: []dynamic.Server{
	// 			{
	// 				URL: "http://localhost:9090",
	// 			},
	// 		},
	// 		PassHostHeader: boolPtr(true),
	// 	},
	// }

	// if date.Minute()%2 == 0 {
	// 	configuration.HTTP.Routers["pp-route-02"] = &dynamic.Router{
	// 		EntryPoints: []string{"web"},
	// 		Service:     "pp-service-02",
	// 		Rule:        "Host(`another.example.com`)",
	// 	}

	// 	configuration.HTTP.Services["pp-service-02"] = &dynamic.Service{
	// 		LoadBalancer: &dynamic.ServersLoadBalancer{
	// 			Servers: []dynamic.Server{
	// 				{
	// 					URL: "http://localhost:9091",
	// 				},
	// 			},
	// 			PassHostHeader: boolPtr(true),
	// 		},
	// 	}
	// }

	return configuration
}

// func boolPtr(v bool) *bool {
// 	return &v
// }
