package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NX211/traefik-proxmox-provider/internal"
	"github.com/traefik/genconf/dynamic"
)

// newMockPVE spins up a fake Proxmox API serving a single node with a single
// running VM labeled traefik.enable=true.
func newMockPVE(t *testing.T, nodeName, release, vmName, vmIP string) *httptest.Server {
	t.Helper()

	writeJSON := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, body)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, fmt.Sprintf(`{"data":{"release":"%s"}}`, release))
	})
	mux.HandleFunc("/api2/json/nodes", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, fmt.Sprintf(`{"data":[{"node":"%s"}]}`, nodeName))
	})
	mux.HandleFunc(fmt.Sprintf("/api2/json/nodes/%s/qemu", nodeName), func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, fmt.Sprintf(`{"data":[{"vmid":100,"name":"%s","status":"running"}]}`, vmName))
	})
	mux.HandleFunc(fmt.Sprintf("/api2/json/nodes/%s/lxc", nodeName), func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"data":[]}`)
	})
	mux.HandleFunc(fmt.Sprintf("/api2/json/nodes/%s/qemu/100/config", nodeName), func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"data":{"description":"traefik.enable=true"}}`)
	})
	mux.HandleFunc(fmt.Sprintf("/api2/json/nodes/%s/qemu/100/agent/network-get-interfaces", nodeName), func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, fmt.Sprintf(`{"data":{"result":[{"ip-addresses":[{"ip-address":"%s","ip-address-type":"ipv4","prefix":24}]}]}}`, vmIP))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(name, endpoint string) *namedClient {
	return &namedClient{
		name:   name,
		client: internal.NewProxmoxClient(endpoint, "test@pam!test", "token", false, "info"),
	}
}

func TestNormalizeNodes(t *testing.T) {
	t.Run("legacy flat config wrapped as single node", func(t *testing.T) {
		config := &Config{
			PollInterval:   "5s",
			ApiEndpoint:    "https://proxmox.example.com",
			ApiTokenId:     "test@pam!test",
			ApiToken:       "test-token",
			ApiLogging:     "debug",
			ApiValidateSSL: "false",
		}
		nodes := normalizeNodes(config)
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}
		if nodes[0].Name != "default" {
			t.Errorf("Expected name 'default', got %s", nodes[0].Name)
		}
		if nodes[0].ApiEndpoint != "https://proxmox.example.com" {
			t.Errorf("Expected endpoint from flat config, got %s", nodes[0].ApiEndpoint)
		}
		if nodes[0].ApiLogging != "debug" {
			t.Errorf("Expected logging 'debug', got %s", nodes[0].ApiLogging)
		}
		if nodes[0].ApiValidateSSL != "false" {
			t.Errorf("Expected SSL validation 'false', got %s", nodes[0].ApiValidateSSL)
		}
	})

	t.Run("multi-node config returned as-is", func(t *testing.T) {
		config := &Config{
			PollInterval: "5s",
			Nodes: []NodeConfig{
				{Name: "proxmox", ApiEndpoint: "https://pve1.example.com", ApiTokenId: "a", ApiToken: "1"},
				{Name: "minimox", ApiEndpoint: "https://pve2.example.com", ApiTokenId: "b", ApiToken: "2"},
			},
		}
		nodes := normalizeNodes(config)
		if len(nodes) != 2 {
			t.Fatalf("Expected 2 nodes, got %d", len(nodes))
		}
		if nodes[0].Name != "proxmox" || nodes[1].Name != "minimox" {
			t.Errorf("Expected names to be preserved, got %s and %s", nodes[0].Name, nodes[1].Name)
		}
	})

	t.Run("defaults filled in for sparse entries", func(t *testing.T) {
		config := &Config{
			PollInterval: "5s",
			Nodes: []NodeConfig{
				{ApiEndpoint: "https://pve1.example.com", ApiTokenId: "a", ApiToken: "1"},
			},
		}
		nodes := normalizeNodes(config)
		if nodes[0].Name != "node-0" {
			t.Errorf("Expected auto-generated name 'node-0', got %s", nodes[0].Name)
		}
		if nodes[0].ApiLogging != "info" {
			t.Errorf("Expected default logging 'info', got %s", nodes[0].ApiLogging)
		}
		if nodes[0].ApiValidateSSL != "true" {
			t.Errorf("Expected default SSL validation 'true', got %s", nodes[0].ApiValidateSSL)
		}
	})
}

func TestValidateConfigMultiNode(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid multi-node config",
			config: &Config{
				PollInterval: "5s",
				Nodes: []NodeConfig{
					{Name: "proxmox", ApiEndpoint: "https://pve1.example.com", ApiTokenId: "a", ApiToken: "1"},
					{Name: "minimox", ApiEndpoint: "https://pve2.example.com", ApiTokenId: "b", ApiToken: "2"},
				},
			},
			wantErr: false,
		},
		{
			name: "entry missing endpoint is fatal",
			config: &Config{
				PollInterval: "5s",
				Nodes: []NodeConfig{
					{Name: "proxmox", ApiEndpoint: "https://pve1.example.com", ApiTokenId: "a", ApiToken: "1"},
					{Name: "minimox", ApiTokenId: "b", ApiToken: "2"},
				},
			},
			wantErr: true,
		},
		{
			name: "entry missing token is fatal",
			config: &Config{
				PollInterval: "5s",
				Nodes: []NodeConfig{
					{Name: "proxmox", ApiEndpoint: "https://pve1.example.com", ApiTokenId: "a"},
				},
			},
			wantErr: true,
		},
		{
			name: "nodes take precedence over incomplete flat fields",
			config: &Config{
				PollInterval: "5s",
				Nodes: []NodeConfig{
					{Name: "proxmox", ApiEndpoint: "https://pve1.example.com", ApiTokenId: "a", ApiToken: "1"},
				},
			},
			wantErr: false,
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

func TestNewSurvivesUnreachableEndpoint(t *testing.T) {
	live := newMockPVE(t, "pve1", "8.4", "web", "10.0.0.5")

	config := &Config{
		PollInterval: "5s",
		Nodes: []NodeConfig{
			{Name: "proxmox", ApiEndpoint: live.URL, ApiTokenId: "a", ApiToken: "1"},
			// 127.0.0.1:1 refuses connections immediately
			{Name: "minimox", ApiEndpoint: "http://127.0.0.1:1", ApiTokenId: "b", ApiToken: "2"},
		},
	}

	p, err := New(context.Background(), config, "test-provider")
	if err != nil {
		t.Fatalf("New() should not fail when an endpoint is unreachable, got: %v", err)
	}
	if len(p.clients) != 2 {
		t.Errorf("Expected 2 clients (down endpoint retried on next poll), got %d", len(p.clients))
	}
}

func TestGetServiceMapsPartialFailure(t *testing.T) {
	live := newMockPVE(t, "pve1", "8.4", "web", "10.0.0.5")

	clients := []*namedClient{
		newTestClient("proxmox", live.URL),
		newTestClient("minimox", "http://127.0.0.1:1"),
	}

	maps := getServiceMaps(clients, context.Background())
	if len(maps) != 1 {
		t.Fatalf("Expected 1 cluster map (dead endpoint skipped), got %d", len(maps))
	}
	if maps[0].clusterName != "proxmox" {
		t.Errorf("Expected surviving map to be 'proxmox', got %s", maps[0].clusterName)
	}
	if len(maps[0].services["pve1"]) != 1 {
		t.Errorf("Expected 1 service discovered on pve1, got %d", len(maps[0].services["pve1"]))
	}
}

func TestUpdateConfigurationPartialFailureKeepsOtherRoutes(t *testing.T) {
	live := newMockPVE(t, "pve1", "8.4", "web", "10.0.0.5")

	p := &Provider{
		name: "test-provider",
		clients: []*namedClient{
			newTestClient("proxmox", live.URL),
			newTestClient("minimox", "http://127.0.0.1:1"),
		},
	}

	cfgChan := make(chan json.Marshaler, 1)
	if err := p.updateConfiguration(context.Background(), cfgChan); err != nil {
		t.Fatalf("updateConfiguration() should succeed with one endpoint down, got: %v", err)
	}

	payload := (<-cfgChan).(*dynamic.JSONPayload)
	// Two configured endpoints -> auto-generated IDs are prefixed.
	if _, ok := payload.HTTP.Routers["proxmox-web-100"]; !ok {
		t.Errorf("Expected router 'proxmox-web-100' from live endpoint, got routers: %v", routerKeys(payload.Configuration))
	}
}

func TestUpdateConfigurationAllEndpointsDown(t *testing.T) {
	p := &Provider{
		name: "test-provider",
		clients: []*namedClient{
			newTestClient("proxmox", "http://127.0.0.1:1"),
			newTestClient("minimox", "http://127.0.0.1:1"),
		},
	}

	cfgChan := make(chan json.Marshaler, 1)
	err := p.updateConfiguration(context.Background(), cfgChan)
	if err == nil {
		t.Fatal("updateConfiguration() should fail when every endpoint is down (so the previous config is kept)")
	}
	select {
	case <-cfgChan:
		t.Error("No configuration should be pushed when every endpoint is down")
	default:
	}
}

func TestGenerateConfigurationMultiEndpointPrefixing(t *testing.T) {
	// Same VM name and ID on two standalone endpoints must not collide.
	clusterMaps := []clusterServiceMap{
		{
			clusterName: "proxmox",
			services: map[string][]internal.Service{
				"pve1": {{
					ID:     100,
					Name:   "web",
					IPs:    []internal.IP{{Address: "10.0.0.5", AddressType: "ipv4"}},
					Config: map[string]string{"traefik.enable": "true"},
				}},
			},
		},
		{
			clusterName: "minimox",
			services: map[string][]internal.Service{
				"pve2": {{
					ID:     100,
					Name:   "web",
					IPs:    []internal.IP{{Address: "10.0.1.5", AddressType: "ipv4"}},
					Config: map[string]string{"traefik.enable": "true"},
				}},
			},
		},
	}

	config := generateConfiguration(clusterMaps, true)

	for _, id := range []string{"proxmox-web-100", "minimox-web-100"} {
		if _, ok := config.HTTP.Routers[id]; !ok {
			t.Errorf("Expected router %q, got: %v", id, routerKeys(config))
		}
		if _, ok := config.HTTP.Services[id]; !ok {
			t.Errorf("Expected service %q", id)
		}
	}

	svc1 := config.HTTP.Services["proxmox-web-100"]
	svc2 := config.HTTP.Services["minimox-web-100"]
	if svc1 == nil || svc2 == nil {
		t.Fatal("Expected both services to exist")
	}
	if svc1.LoadBalancer.Servers[0].URL == svc2.LoadBalancer.Servers[0].URL {
		t.Errorf("Services should point at different backends, both got %s", svc1.LoadBalancer.Servers[0].URL)
	}
}

func TestGenerateConfigurationSingleEndpointNoPrefix(t *testing.T) {
	clusterMaps := []clusterServiceMap{
		{
			clusterName: "default",
			services: map[string][]internal.Service{
				"pve1": {{
					ID:     100,
					Name:   "web",
					IPs:    []internal.IP{{Address: "10.0.0.5", AddressType: "ipv4"}},
					Config: map[string]string{"traefik.enable": "true"},
				}},
			},
		},
	}

	config := generateConfiguration(clusterMaps, false)

	if _, ok := config.HTTP.Routers["web-100"]; !ok {
		t.Errorf("Auto-generated IDs must stay unprefixed in single-endpoint mode, got: %v", routerKeys(config))
	}
}

func TestGenerateConfigurationExplicitNamesNeverPrefixed(t *testing.T) {
	clusterMaps := []clusterServiceMap{
		{
			clusterName: "proxmox",
			services: map[string][]internal.Service{
				"pve1": {{
					ID:   100,
					Name: "web",
					IPs:  []internal.IP{{Address: "10.0.0.5", AddressType: "ipv4"}},
					Config: map[string]string{
						"traefik.enable":                  "true",
						"traefik.http.routers.myapp.rule": "Host(`myapp.example.com`)",
					},
				}},
			},
		},
	}

	config := generateConfiguration(clusterMaps, true)

	if _, ok := config.HTTP.Routers["myapp"]; !ok {
		t.Errorf("Explicit router names must not be prefixed, got: %v", routerKeys(config))
	}
}

func TestMaybeLogVersion(t *testing.T) {
	t.Run("marks endpoint logged on success", func(t *testing.T) {
		live := newMockPVE(t, "pve1", "8.4", "web", "10.0.0.5")
		nc := newTestClient("proxmox", live.URL)

		maybeLogVersion(nc, context.Background())
		if !nc.versionLogged {
			t.Error("Expected versionLogged to be true after successful version fetch")
		}
	})

	t.Run("keeps retrying while endpoint is down", func(t *testing.T) {
		nc := newTestClient("minimox", "http://127.0.0.1:1")

		maybeLogVersion(nc, context.Background())
		if nc.versionLogged {
			t.Error("Expected versionLogged to stay false when endpoint is unreachable")
		}
	})
}

// routerKeys is a test helper for readable error messages.
func routerKeys(cfg *dynamic.Configuration) []string {
	result := make([]string, 0, len(cfg.HTTP.Routers))
	for k := range cfg.HTTP.Routers {
		result = append(result, k)
	}
	return result
}
