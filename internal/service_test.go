package internal

import (
	"testing"
)

func TestService_Basics(t *testing.T) {
	// Create a service directly without calling NewService
	service := Service{
		ID:     100,
		Name:   "test-service",
		Config: map[string]string{"traefik.enable": "true"},
		IPs:    make([]IP, 0),
	}
	
	// Test basic properties
	if service.ID != 100 {
		t.Errorf("Service ID = %v, want %v", service.ID, 100)
	}
	
	if service.Name != "test-service" {
		t.Errorf("Service Name = %v, want %v", service.Name, "test-service")
	}
	
	if service.Config["traefik.enable"] != "true" {
		t.Errorf("Config value = %v, want %v", service.Config["traefik.enable"], "true")
	}
	
	if len(service.IPs) != 0 {
		t.Errorf("Expected empty IPs, got %d items", len(service.IPs))
	}
}

func TestService_Config(t *testing.T) {
	// Create services directly
	serviceWithEnable := Service{
		ID:     1,
		Name:   "enabled-service",
		Config: map[string]string{"traefik.enable": "true"},
		IPs:    make([]IP, 0),
	}
	
	enableValue, exists := serviceWithEnable.Config["traefik.enable"]
	if !exists {
		t.Error("Expected 'traefik.enable' config to exist but it doesn't")
	}
	if enableValue != "true" {
		t.Errorf("Config value = %v, want %v", enableValue, "true")
	}
	
	// Test with empty config
	serviceWithEmptyConfig := Service{
		ID:     2,
		Name:   "empty-config-service",
		Config: map[string]string{},
		IPs:    make([]IP, 0),
	}
	
	_, exists = serviceWithEmptyConfig.Config["traefik.enable"]
	if exists {
		t.Error("Didn't expect 'traefik.enable' config to exist but it does")
	}
}

func TestService_IPs(t *testing.T) {
	// Create a service with IPs
	service := Service{
		ID:     300,
		Name:   "ip-service",
		Config: map[string]string{},
		IPs: []IP{
			{Address: "192.168.1.1", AddressType: "ipv4", Prefix: 24},
		},
	}
	
	if len(service.IPs) != 1 {
		t.Fatalf("Expected 1 IP, got %d", len(service.IPs))
	}
	
	if service.IPs[0].Address != "192.168.1.1" {
		t.Errorf("Expected IP address 192.168.1.1, got %s", service.IPs[0].Address)
	}
}

func TestParsedConfig_GetTraefikMap(t *testing.T) {
	pc := ParsedConfig{
		Description: "traefik.enable=true\ntraefik.http.routers.test.rule=Host(`test.example.com`)",
	}
	
	m := pc.GetTraefikMap()
	
	if len(m) != 2 {
		t.Errorf("Expected 2 config items, got %d", len(m))
	}
	
	if m["traefik.enable"] != "true" {
		t.Errorf("Expected traefik.enable=true, got %s", m["traefik.enable"])
	}
	
	if m["traefik.http.routers.test.rule"] != "Host(`test.example.com`)" {
		t.Errorf("Expected correct router rule, got %s", m["traefik.http.routers.test.rule"])
	}
}

func TestParsedConfig_GetTraefikMap_SpaceSeparated(t *testing.T) {
	// OCI containers in Proxmox may return description with space-separated labels
	// instead of newline-separated ones.
	pc := ParsedConfig{
		Description: "traefik.enable=true traefik.http.routers.retro.entrypoints=websecure traefik.http.routers.retro.rule=Host(`retro.example.com`) traefik.http.services.retro.loadbalancer.server.port=80",
	}

	m := pc.GetTraefikMap()

	if len(m) != 4 {
		t.Errorf("Expected 4 config items, got %d", len(m))
	}

	if m["traefik.enable"] != "true" {
		t.Errorf("Expected traefik.enable=true, got %s", m["traefik.enable"])
	}

	if m["traefik.http.routers.retro.entrypoints"] != "websecure" {
		t.Errorf("Expected entrypoints=websecure, got %s", m["traefik.http.routers.retro.entrypoints"])
	}

	if m["traefik.http.routers.retro.rule"] != "Host(`retro.example.com`)" {
		t.Errorf("Expected correct router rule, got %s", m["traefik.http.routers.retro.rule"])
	}

	if m["traefik.http.services.retro.loadbalancer.server.port"] != "80" {
		t.Errorf("Expected port=80, got %s", m["traefik.http.services.retro.loadbalancer.server.port"])
	}
}

func TestParsedConfig_GetTraefikMap_MixedSeparators(t *testing.T) {
	// Description with non-traefik text, newlines, and space-separated traefik labels.
	pc := ParsedConfig{
		Description: "My application server\n\ntraefik.enable=true traefik.http.routers.app.rule=Host(`app.example.com`)\ntraefik.http.services.app.loadbalancer.server.port=3000",
	}

	m := pc.GetTraefikMap()

	if len(m) != 3 {
		t.Errorf("Expected 3 config items, got %d", len(m))
	}

	if m["traefik.enable"] != "true" {
		t.Errorf("Expected traefik.enable=true, got %s", m["traefik.enable"])
	}

	if m["traefik.http.routers.app.rule"] != "Host(`app.example.com`)" {
		t.Errorf("Expected correct router rule, got %s", m["traefik.http.routers.app.rule"])
	}

	if m["traefik.http.services.app.loadbalancer.server.port"] != "3000" {
		t.Errorf("Expected port=3000, got %s", m["traefik.http.services.app.loadbalancer.server.port"])
	}
}

func TestParsedAgentInterfaces_GetIPs(t *testing.T) {
	pai := ParsedAgentInterfaces{
		Result: []struct {
			IPAddresses []IP `json:"ip-addresses"`
		}{
			{
				IPAddresses: []IP{
					{Address: "192.168.1.1", AddressType: "ipv4", Prefix: 24},
					{Address: "10.0.0.1", AddressType: "ipv4", Prefix: 16},
				},
			},
		},
	}
	
	ips := pai.GetIPs()
	
	if len(ips) != 2 {
		t.Errorf("Expected 2 IPs, got %d", len(ips))
	}
	
	if ips[0].Address != "192.168.1.1" {
		t.Errorf("Expected first IP to be 192.168.1.1, got %s", ips[0].Address)
	}
	
	if ips[1].Address != "10.0.0.1" {
		t.Errorf("Expected second IP to be 10.0.0.1, got %s", ips[1].Address)
	}
} 