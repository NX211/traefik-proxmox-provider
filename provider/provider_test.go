package provider

import (
	"context"
	"testing"

	"github.com/NX211/traefik-proxmox-provider/internal"
)

func TestProviderConfig(t *testing.T) {
	config := CreateConfig()
	if config.PollInterval != "30s" {
		t.Errorf("Expected default PollInterval to be '30s', got %s", config.PollInterval)
	}
	if config.ApiValidateSSL != "true" {
		t.Errorf("Expected default ApiValidateSSL to be 'true', got %s", config.ApiValidateSSL)
	}
	if config.ApiLogging != "info" {
		t.Errorf("Expected default ApiLogging to be 'info', got %s", config.ApiLogging)
	}
}

func TestProviderNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "Valid config",
			config: &Config{
				PollInterval:   "5s",
				ApiEndpoint:    "https://proxmox.example.com",
				ApiTokenId:     "test@pam!test",
				ApiToken:       "test-token",
				ApiValidateSSL: "true",
				ApiLogging:     "info",
			},
			wantErr: true, // We expect an error because the domain doesn't exist
		},
		{
			name:    "Nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "Missing poll interval",
			config: &Config{
				ApiEndpoint:    "https://proxmox.example.com",
				ApiTokenId:     "test@pam!test",
				ApiToken:       "test-token",
				ApiValidateSSL: "true",
				ApiLogging:     "info",
			},
			wantErr: true,
		},
		{
			name: "Invalid poll interval",
			config: &Config{
				PollInterval:   "invalid",
				ApiEndpoint:    "https://proxmox.example.com",
				ApiTokenId:     "test@pam!test",
				ApiToken:       "test-token",
				ApiValidateSSL: "true",
				ApiLogging:     "info",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(context.Background(), tt.config, "test-provider")
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && provider != nil {
				t.Error("Expected provider to be nil when there's an error")
			}
		})
	}
}

func TestProviderValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "Valid config",
			config: &Config{
				PollInterval:   "5s",
				ApiEndpoint:    "https://proxmox.example.com",
				ApiTokenId:     "test@pam!test",
				ApiToken:       "test-token",
				ApiValidateSSL: "true",
				ApiLogging:     "info",
			},
			wantErr: false,
		},
		{
			name:    "Nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "Missing poll interval",
			config: &Config{
				ApiEndpoint:    "https://proxmox.example.com",
				ApiTokenId:     "test@pam!test",
				ApiToken:       "test-token",
				ApiValidateSSL: "true",
				ApiLogging:     "info",
			},
			wantErr: true,
		},
		{
			name: "Missing endpoint",
			config: &Config{
				PollInterval:   "5s",
				ApiTokenId:     "test@pam!test",
				ApiToken:       "test-token",
				ApiValidateSSL: "true",
				ApiLogging:     "info",
			},
			wantErr: true,
		},
		{
			name: "Missing token ID",
			config: &Config{
				PollInterval:   "5s",
				ApiEndpoint:    "https://proxmox.example.com",
				ApiToken:       "test-token",
				ApiValidateSSL: "true",
				ApiLogging:     "info",
			},
			wantErr: true,
		},
		{
			name: "Missing token",
			config: &Config{
				PollInterval:   "5s",
				ApiEndpoint:    "https://proxmox.example.com",
				ApiTokenId:     "test@pam!test",
				ApiValidateSSL: "true",
				ApiLogging:     "info",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProviderParserConfig(t *testing.T) {
	tests := []struct {
		name        string
		apiEndpoint string
		tokenID     string
		token       string
		wantErr     bool
	}{
		{
			name:        "Valid config",
			apiEndpoint: "https://proxmox.example.com",
			tokenID:     "test@pam!test",
			token:       "test-token",
			wantErr:     false,
		},
		{
			name:        "Missing endpoint",
			apiEndpoint: "",
			tokenID:     "test@pam!test",
			token:       "test-token",
			wantErr:     true,
		},
		{
			name:        "Missing token ID",
			apiEndpoint: "https://proxmox.example.com",
			tokenID:     "",
			token:       "test-token",
			wantErr:     true,
		},
		{
			name:        "Missing token",
			apiEndpoint: "https://proxmox.example.com",
			tokenID:     "test@pam!test",
			token:       "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := newParserConfig(tt.apiEndpoint, tt.tokenID, tt.token, "debug", true)
			if (err != nil) != tt.wantErr {
				t.Errorf("newParserConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if config.ApiEndpoint != tt.apiEndpoint {
					t.Errorf("Expected ApiEndpoint to be %s, got %s", tt.apiEndpoint, config.ApiEndpoint)
				}
				if config.TokenId != tt.tokenID {
					t.Errorf("Expected TokenId to be %s, got %s", tt.tokenID, config.TokenId)
				}
				if config.Token != tt.token {
					t.Errorf("Expected Token to be %s, got %s", tt.token, config.Token)
				}
			}
		})
	}
}

func TestProviderService(t *testing.T) {
	config := map[string]string{
		"traefik.enable":                 "true",
		"traefik.http.routers.test.rule": "Host(`test.example.com`)",
	}

	service := internal.NewService(123, "test-service", config)
	if service.ID != 123 {
		t.Errorf("Expected service ID to be 123, got %d", service.ID)
	}
	if service.Name != "test-service" {
		t.Errorf("Expected service name to be 'test-service', got %s", service.Name)
	}
	if len(service.Config) != 2 {
		t.Errorf("Expected service config to have 2 items, got %d", len(service.Config))
	}
	if len(service.IPs) != 0 {
		t.Errorf("Expected service IPs to be empty, got %d items", len(service.IPs))
	}
}

func TestGetServiceURL(t *testing.T) {
	tests := []struct {
		name        string
		service     internal.Service
		serviceName string
		nodeName    string
		expectedUrl string
	}{
		{
			name:        "IP set, default port and scheme",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.ip": "1.2.3.4",
				},
			},
			expectedUrl: "http://1.2.3.4:80",
		},
		{
			name:        "IP and scheme set, default port (http)",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.ip":     "1.2.3.4",
					"traefik.http.services.service.loadbalancer.server.scheme": "http",
				},
			},
			expectedUrl: "http://1.2.3.4:80",
		},
		{
			name:        "IP and scheme set, default port (https)",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.ip":     "1.2.3.4",
					"traefik.http.services.service.loadbalancer.server.scheme": "https",
				},
			},
			expectedUrl: "https://1.2.3.4:443",
		},
		{
			name:        "IP, port and scheme set",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.ip":     "1.2.3.4",
					"traefik.http.services.service.loadbalancer.server.scheme": "https",
					"traefik.http.services.service.loadbalancer.server.port":   "8080",
				},
			},
			expectedUrl: "https://1.2.3.4:8080",
		},
		{
			name:        "URL is set",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.url": "http://test.com:1234",
				},
			},
			expectedUrl: "http://test.com:1234",
		},
		{
			name:        "URL overrides other settings",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.url":    "http://test.com:1234",
					"traefik.http.services.service.loadbalancer.server.ip":     "1.2.3.4",
					"traefik.http.services.service.loadbalancer.server.scheme": "https",
					"traefik.http.services.service.loadbalancer.server.port":   "8080",
				},
			},
			expectedUrl: "http://test.com:1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := getServiceURL(tt.service, tt.serviceName, tt.nodeName)
			if url != tt.expectedUrl {
				t.Errorf("Expected URL to be %s, got %s", tt.expectedUrl, url)
			}
		})
	}
}
