package provider

import (
	"context"
	"testing"

	"github.com/NX211/traefik-proxmox-provider/internal"
	"github.com/traefik/genconf/dynamic"
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

func TestGetServiceURLs(t *testing.T) {
	tests := []struct {
		name         string
		service      internal.Service
		serviceName  string
		nodeName     string
		expectedURLs []string
	}{
		{
			name:        "IP label set, default port and scheme",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.ip": "1.2.3.4",
				},
			},
			expectedURLs: []string{"http://1.2.3.4:80"},
		},
		{
			name:        "IP label and scheme=http set",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.ip":     "1.2.3.4",
					"traefik.http.services.service.loadbalancer.server.scheme": "http",
				},
			},
			expectedURLs: []string{"http://1.2.3.4:80"},
		},
		{
			name:        "IP label and scheme=https set, default port",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.ip":     "1.2.3.4",
					"traefik.http.services.service.loadbalancer.server.scheme": "https",
				},
			},
			expectedURLs: []string{"https://1.2.3.4:443"},
		},
		{
			name:        "IP label, port and scheme set",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.ip":     "1.2.3.4",
					"traefik.http.services.service.loadbalancer.server.scheme": "https",
					"traefik.http.services.service.loadbalancer.server.port":   "8080",
				},
			},
			expectedURLs: []string{"https://1.2.3.4:8080"},
		},
		{
			name:        "URL label set",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.url": "http://test.com:1234",
				},
			},
			expectedURLs: []string{"http://test.com:1234"},
		},
		{
			name:        "URL label overrides everything else",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.url":    "http://test.com:1234",
					"traefik.http.services.service.loadbalancer.server.ip":     "1.2.3.4",
					"traefik.http.services.service.loadbalancer.server.scheme": "https",
					"traefik.http.services.service.loadbalancer.server.port":   "8080",
				},
			},
			expectedURLs: []string{"http://test.com:1234"},
		},
		{
			name:        "single discovered IP",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{},
				IPs: []internal.IP{
					{Address: "10.0.0.5", AddressType: "ipv4"},
				},
			},
			expectedURLs: []string{"http://10.0.0.5:80"},
		},
		{
			name:        "multiple discovered IPs fan out and sort",
			serviceName: "service",
			service: internal.Service{
				Config: map[string]string{
					"traefik.http.services.service.loadbalancer.server.port": "8080",
				},
				// Deliberately reversed to verify sorting kicks in.
				IPs: []internal.IP{
					{Address: "192.168.1.20", AddressType: "ipv4"},
					{Address: "10.0.0.5", AddressType: "ipv4"},
				},
			},
			expectedURLs: []string{"http://10.0.0.5:8080", "http://192.168.1.20:8080"},
		},
		{
			name:        "hostname fallback when no IPs",
			serviceName: "service",
			service: internal.Service{
				ID:     42,
				Name:   "myvm",
				Config: map[string]string{},
			},
			nodeName:     "pve1",
			expectedURLs: []string{"http://myvm.pve1:80"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls := getServiceURLs(tt.service, tt.serviceName, tt.nodeName, false)
			if len(urls) != len(tt.expectedURLs) {
				t.Fatalf("len(urls) = %d, want %d. Got %v", len(urls), len(tt.expectedURLs), urls)
			}
			for i, want := range tt.expectedURLs {
				if urls[i] != want {
					t.Errorf("urls[%d] = %q, want %q", i, urls[i], want)
				}
			}
		})
	}
}

func TestHandleRouterTLS_ArrayDomains(t *testing.T) {
	tests := []struct {
		name           string
		config         map[string]string
		expectedMain   []string
		expectedSANs   [][]string
		expectNil      bool
	}{
		{
			name: "Array syntax with main and sans",
			config: map[string]string{
				"traefik.http.routers.test.tls.domains[0].main": "example.com",
				"traefik.http.routers.test.tls.domains[0].sans": "*.example.com,www.example.com",
				"traefik.http.routers.test.tls.domains[1].main": "another.com",
			},
			expectedMain: []string{"example.com", "another.com"},
			expectedSANs: [][]string{{"*.example.com", "www.example.com"}, nil},
		},
		{
			name: "Simple domains fallback",
			config: map[string]string{
				"traefik.http.routers.test.tls.domains": "example.com,another.com",
			},
			expectedMain: []string{"example.com", "another.com"},
			expectedSANs: [][]string{nil, nil},
		},
		{
			name:      "No TLS config",
			config:    map[string]string{},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := internal.Service{Config: tt.config}
			tlsConfig := handleRouterTLS(service, "traefik.http.routers.test")

			if tt.expectNil {
				if tlsConfig != nil {
					t.Error("Expected nil TLS config")
				}
				return
			}

			if tlsConfig == nil {
				t.Fatal("Expected non-nil TLS config")
			}

			if len(tlsConfig.Domains) != len(tt.expectedMain) {
				t.Fatalf("Expected %d domains, got %d", len(tt.expectedMain), len(tlsConfig.Domains))
			}

			for i, domain := range tlsConfig.Domains {
				if domain.Main != tt.expectedMain[i] {
					t.Errorf("Domain[%d].Main = %s, want %s", i, domain.Main, tt.expectedMain[i])
				}
				if tt.expectedSANs[i] != nil {
					if len(domain.SANs) != len(tt.expectedSANs[i]) {
						t.Errorf("Domain[%d].SANs length = %d, want %d", i, len(domain.SANs), len(tt.expectedSANs[i]))
					}
				}
			}
		})
	}
}

// makeServiceMap is a tiny helper for generateConfiguration table tests.
func makeServiceMap(node string, services ...internal.Service) map[string][]internal.Service {
	return map[string][]internal.Service{node: services}
}

func TestGenerateConfiguration_DefaultIDNamespacing(t *testing.T) {
	// VMID 101, name "web", only `enable=true` — no router/service labels.
	// Expect a single router and service both keyed "web-101" (defaultID).
	svc := internal.Service{
		ID:   101,
		Name: "web",
		Config: map[string]string{
			"traefik.enable": "true",
		},
	}
	cfg := generateConfiguration(makeServiceMap("pve1", svc), false)

	if _, ok := cfg.HTTP.Routers["web-101"]; !ok {
		t.Errorf("Routers[web-101] missing. Got keys: %v", routerKeys(cfg))
	}
	if _, ok := cfg.HTTP.Services["web-101"]; !ok {
		t.Errorf("Services[web-101] missing. Got keys: %v", serviceKeys(cfg))
	}
	// Default rule is Host(`<name>`)
	if got := cfg.HTTP.Routers["web-101"].Rule; got != "Host(`web`)" {
		t.Errorf("Rule = %q, want Host(`web`)", got)
	}
	// Default Service link points at the same defaultID (no extra suffix).
	if got := cfg.HTTP.Routers["web-101"].Service; got != "web-101" {
		t.Errorf("Service = %q, want web-101", got)
	}
	// Priority must NOT be hardcoded — Traefik default sort relies on 0.
	if got := cfg.HTTP.Routers["web-101"].Priority; got != 0 {
		t.Errorf("Priority = %d, want 0 (omitempty so Traefik default applies)", got)
	}
}

func TestGenerateConfiguration_LabelNamespacing(t *testing.T) {
	// User-supplied `app` router and service on VMID 101 must be emitted as `app-101`.
	svc := internal.Service{
		ID:   101,
		Name: "web",
		Config: map[string]string{
			"traefik.enable":                                          "true",
			"traefik.http.routers.app.rule":                           "Host(`app.example.com`)",
			"traefik.http.services.app.loadbalancer.server.port":      "8080",
		},
	}
	cfg := generateConfiguration(makeServiceMap("pve1", svc), false)

	if _, ok := cfg.HTTP.Routers["app-101"]; !ok {
		t.Fatalf("Routers[app-101] missing. Got: %v", routerKeys(cfg))
	}
	if _, ok := cfg.HTTP.Services["app-101"]; !ok {
		t.Fatalf("Services[app-101] missing. Got: %v", serviceKeys(cfg))
	}
	// Bare "app" must NOT exist.
	if _, ok := cfg.HTTP.Routers["app"]; ok {
		t.Errorf("Routers[app] should not exist after namespacing")
	}
	// Default cross-link points at the namespaced service.
	if got := cfg.HTTP.Routers["app-101"].Service; got != "app-101" {
		t.Errorf("Service link = %q, want app-101", got)
	}
}

func TestGenerateConfiguration_CrossReferenceRewrite(t *testing.T) {
	// Router r1 explicitly references service svc1 on the same guest.
	// After namespacing, r1 should point at svc1-200, not svc1.
	svc := internal.Service{
		ID:   200,
		Name: "host",
		Config: map[string]string{
			"traefik.enable":                                       "true",
			"traefik.http.routers.r1.rule":                         "Host(`r1.example.com`)",
			"traefik.http.routers.r1.service":                      "svc1",
			"traefik.http.services.svc1.loadbalancer.server.port":  "8080",
		},
	}
	cfg := generateConfiguration(makeServiceMap("pve1", svc), false)

	r, ok := cfg.HTTP.Routers["r1-200"]
	if !ok {
		t.Fatalf("Routers[r1-200] missing")
	}
	if r.Service != "svc1-200" {
		t.Errorf("r1-200.Service = %q, want svc1-200", r.Service)
	}
}

func TestGenerateConfiguration_PreservesForeignServiceRef(t *testing.T) {
	// Router r1 references a service that is NOT declared on this guest.
	// Such references (typically `name@file` from the file provider) must
	// pass through unchanged.
	svc := internal.Service{
		ID:   101,
		Name: "host",
		Config: map[string]string{
			"traefik.enable":                  "true",
			"traefik.http.routers.r1.rule":    "Host(`r1.example.com`)",
			"traefik.http.routers.r1.service": "external@file",
		},
	}
	cfg := generateConfiguration(makeServiceMap("pve1", svc), false)

	r, ok := cfg.HTTP.Routers["r1-101"]
	if !ok {
		t.Fatalf("Routers[r1-101] missing")
	}
	if r.Service != "external@file" {
		t.Errorf("Service = %q, want external@file (foreign refs must not be rewritten)", r.Service)
	}
}

func TestGenerateConfiguration_NoCollisionAcrossGuests(t *testing.T) {
	// Two guests, both declaring router/service "app". Without namespacing
	// they collide and the second wins silently. With namespacing both
	// must be present and distinct.
	svc100 := internal.Service{
		ID:   100,
		Name: "g1",
		Config: map[string]string{
			"traefik.enable":                                     "true",
			"traefik.http.routers.app.rule":                      "Host(`a.example.com`)",
			"traefik.http.services.app.loadbalancer.server.port": "8080",
		},
	}
	svc200 := internal.Service{
		ID:   200,
		Name: "g2",
		Config: map[string]string{
			"traefik.enable":                                     "true",
			"traefik.http.routers.app.rule":                      "Host(`b.example.com`)",
			"traefik.http.services.app.loadbalancer.server.port": "9090",
		},
	}
	cfg := generateConfiguration(makeServiceMap("pve1", svc100, svc200), false)

	for _, key := range []string{"app-100", "app-200"} {
		if _, ok := cfg.HTTP.Routers[key]; !ok {
			t.Errorf("Routers[%s] missing", key)
		}
		if _, ok := cfg.HTTP.Services[key]; !ok {
			t.Errorf("Services[%s] missing", key)
		}
	}
	if got := cfg.HTTP.Routers["app-100"].Rule; got != "Host(`a.example.com`)" {
		t.Errorf("app-100.Rule = %q, want Host(`a.example.com`)", got)
	}
	if got := cfg.HTTP.Routers["app-200"].Rule; got != "Host(`b.example.com`)" {
		t.Errorf("app-200.Rule = %q, want Host(`b.example.com`)", got)
	}
}

func TestGenerateConfiguration_DottedNameSkipped(t *testing.T) {
	// `routers.my.app.rule` is ambiguous and must be rejected. The guest
	// has no other router labels, so it falls back to the defaultID path.
	svc := internal.Service{
		ID:   50,
		Name: "host",
		Config: map[string]string{
			"traefik.enable":                  "true",
			"traefik.http.routers.my.app.rule": "Host(`a.example.com`)",
		},
	}
	cfg := generateConfiguration(makeServiceMap("pve1", svc), false)

	// No router named my.app, my-50, or my.app-50 should exist.
	for _, k := range []string{"my", "my-50", "my.app", "my.app-50"} {
		if _, ok := cfg.HTTP.Routers[k]; ok {
			t.Errorf("Routers[%s] exists; dotted-name labels should be skipped entirely. Got keys %v", k, routerKeys(cfg))
		}
	}
	// Default-ID router should exist (host-50) since no valid labels were
	// extracted but traefik.enable=true.
	if _, ok := cfg.HTTP.Routers["host-50"]; !ok {
		t.Errorf("Routers[host-50] (defaultID) missing. Got %v", routerKeys(cfg))
	}
}

func TestGenerateConfiguration_MultiIPFanout(t *testing.T) {
	svc := internal.Service{
		ID:   101,
		Name: "web",
		Config: map[string]string{
			"traefik.enable":                                     "true",
			"traefik.http.routers.app.rule":                      "Host(`app.example.com`)",
			"traefik.http.services.app.loadbalancer.server.port": "8080",
		},
		IPs: []internal.IP{
			{Address: "192.168.1.20", AddressType: "ipv4"},
			{Address: "10.0.0.5", AddressType: "ipv4"},
		},
	}
	cfg := generateConfiguration(makeServiceMap("pve1", svc), false)

	servers := cfg.HTTP.Services["app-101"].LoadBalancer.Servers
	if len(servers) != 2 {
		t.Fatalf("len(Servers) = %d, want 2 (multi-IP fanout)", len(servers))
	}
	// Sorted lexicographically by URL.
	want := []string{"http://10.0.0.5:8080", "http://192.168.1.20:8080"}
	for i, w := range want {
		if servers[i].URL != w {
			t.Errorf("Servers[%d].URL = %q, want %q", i, servers[i].URL, w)
		}
	}
}

// routerKeys / serviceKeys collect map keys for assertion error messages.
func routerKeys(cfg interface{ /* dynamic.Configuration */
}) []string {
	c := cfg.(*dynamic.Configuration)
	out := make([]string, 0, len(c.HTTP.Routers))
	for k := range c.HTTP.Routers {
		out = append(out, k)
	}
	return out
}

func serviceKeys(cfg interface{ /* dynamic.Configuration */
}) []string {
	c := cfg.(*dynamic.Configuration)
	out := make([]string, 0, len(c.HTTP.Services))
	for k := range c.HTTP.Services {
		out = append(out, k)
	}
	return out
}

func TestExtractLabelName(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		prefix  string
		allow   map[string]bool
		want    string
		wantOk  bool
	}{
		{"router rule", "traefik.http.routers.r1.rule", "traefik.http.routers.", routerSubkeyAllowList, "r1", true},
		{"router tls.options", "traefik.http.routers.r1.tls.options", "traefik.http.routers.", routerSubkeyAllowList, "r1", true},
		{"router tls.domains[0].main", "traefik.http.routers.r1.tls.domains[0].main", "traefik.http.routers.", routerSubkeyAllowList, "r1", true},
		{"router named tls", "traefik.http.routers.tls.rule", "traefik.http.routers.", routerSubkeyAllowList, "tls", true},
		{"router named priority", "traefik.http.routers.priority.rule", "traefik.http.routers.", routerSubkeyAllowList, "priority", true},
		{"dotted router name", "traefik.http.routers.my.app.rule", "traefik.http.routers.", routerSubkeyAllowList, "", false},
		{"dotted router name two", "traefik.http.routers.api.v2.middlewares", "traefik.http.routers.", routerSubkeyAllowList, "", false},
		{"router no subkey", "traefik.http.routers.r1", "traefik.http.routers.", routerSubkeyAllowList, "", false},
		{"router empty name", "traefik.http.routers..rule", "traefik.http.routers.", routerSubkeyAllowList, "", false},
		{"unknown prefix", "traefik.http.middlewares.m1.headers", "traefik.http.routers.", routerSubkeyAllowList, "", false},

		{"service loadbalancer.server.port", "traefik.http.services.svc1.loadbalancer.server.port", "traefik.http.services.", serviceSubkeyAllowList, "svc1", true},
		{"dotted service name", "traefik.http.services.my.svc.loadbalancer.server.port", "traefik.http.services.", serviceSubkeyAllowList, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractLabelName(tt.key, tt.prefix, tt.allow)
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyRouterOptions(t *testing.T) {
	t.Run("entrypoints plural", func(t *testing.T) {
		svc := internal.Service{Config: map[string]string{
			"traefik.http.routers.r1.entrypoints": "web,websecure",
		}}
		var r dynamic.Router
		applyRouterOptions(&r, svc, "r1")
		if len(r.EntryPoints) != 2 || r.EntryPoints[0] != "web" || r.EntryPoints[1] != "websecure" {
			t.Errorf("EntryPoints = %v, want [web websecure]", r.EntryPoints)
		}
	})
	t.Run("entrypoint singular", func(t *testing.T) {
		svc := internal.Service{Config: map[string]string{
			"traefik.http.routers.r1.entrypoint": "web",
		}}
		var r dynamic.Router
		applyRouterOptions(&r, svc, "r1")
		if len(r.EntryPoints) != 1 || r.EntryPoints[0] != "web" {
			t.Errorf("EntryPoints = %v, want [web]", r.EntryPoints)
		}
	})
	t.Run("middlewares", func(t *testing.T) {
		svc := internal.Service{Config: map[string]string{
			"traefik.http.routers.r1.middlewares": "auth@file,compression",
		}}
		var r dynamic.Router
		applyRouterOptions(&r, svc, "r1")
		if len(r.Middlewares) != 2 || r.Middlewares[0] != "auth@file" || r.Middlewares[1] != "compression" {
			t.Errorf("Middlewares = %v, want [auth@file compression]", r.Middlewares)
		}
	})
	t.Run("priority valid int", func(t *testing.T) {
		svc := internal.Service{Config: map[string]string{
			"traefik.http.routers.r1.priority": "42",
		}}
		var r dynamic.Router
		applyRouterOptions(&r, svc, "r1")
		if r.Priority != 42 {
			t.Errorf("Priority = %d, want 42", r.Priority)
		}
	})
	t.Run("priority invalid leaves zero", func(t *testing.T) {
		svc := internal.Service{Config: map[string]string{
			"traefik.http.routers.r1.priority": "notanint",
		}}
		var r dynamic.Router
		applyRouterOptions(&r, svc, "r1")
		if r.Priority != 0 {
			t.Errorf("Priority = %d, want 0 (parse failure must leave zero)", r.Priority)
		}
	})
	t.Run("no priority label leaves zero", func(t *testing.T) {
		svc := internal.Service{Config: map[string]string{}}
		var r dynamic.Router
		applyRouterOptions(&r, svc, "r1")
		if r.Priority != 0 {
			t.Errorf("Priority = %d, want 0 (no label)", r.Priority)
		}
	})
}

func TestMapKeysToSlice_Sorted(t *testing.T) {
	in := map[string]bool{"charlie": true, "alpha": true, "bravo": true, "delta": true}
	got := mapKeysToSlice(in)
	want := []string{"alpha", "bravo", "charlie", "delta"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("got[%d] = %q, want %q (full got = %v)", i, got[i], v, got)
		}
	}
}

func TestIsBoolLabelEnabled(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   bool
	}{
		{"missing key", map[string]string{}, false},
		{"empty value", map[string]string{"traefik.enable": ""}, false},
		{"true literal", map[string]string{"traefik.enable": "true"}, true},
		{"True mixed case", map[string]string{"traefik.enable": "True"}, true},
		{"TRUE upper", map[string]string{"traefik.enable": "TRUE"}, true},
		{"t shorthand", map[string]string{"traefik.enable": "t"}, true},
		{"T shorthand", map[string]string{"traefik.enable": "T"}, true},
		{"1 numeric", map[string]string{"traefik.enable": "1"}, true},
		{"false literal", map[string]string{"traefik.enable": "false"}, false},
		{"0 numeric", map[string]string{"traefik.enable": "0"}, false},
		{"f shorthand", map[string]string{"traefik.enable": "f"}, false},
		// Intentionally NOT supported (Traefik core doesn't accept these):
		{"yes rejected", map[string]string{"traefik.enable": "yes"}, false},
		{"on rejected", map[string]string{"traefik.enable": "on"}, false},
		{"junk rejected", map[string]string{"traefik.enable": "blarg"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBoolLabelEnabled(tt.labels, "traefik.enable")
			if got != tt.want {
				t.Errorf("isBoolLabelEnabled(%v) = %v, want %v", tt.labels, got, tt.want)
			}
		})
	}
}
