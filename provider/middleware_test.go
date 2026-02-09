package provider

import (
	"testing"

	"github.com/NX211/traefik-proxmox-provider/internal"
)

func TestBuildMiddleware_RedirectRegex(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-redirect.redirectregex.regex":       "^.*$",
		"traefik.http.middlewares.my-redirect.redirectregex.replacement": "http://192.168.4.165:3000",
		"traefik.http.middlewares.my-redirect.redirectregex.permanent":   "false",
	}
	mw, err := buildMiddleware("redirectregex", labels, "my-redirect")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.RedirectRegex == nil {
		t.Fatal("expected RedirectRegex to be set")
	}
	if mw.RedirectRegex.Regex != "^.*$" {
		t.Errorf("Regex = %q, want %q", mw.RedirectRegex.Regex, "^.*$")
	}
	if mw.RedirectRegex.Replacement != "http://192.168.4.165:3000" {
		t.Errorf("Replacement = %q, want %q", mw.RedirectRegex.Replacement, "http://192.168.4.165:3000")
	}
	if mw.RedirectRegex.Permanent != false {
		t.Error("Permanent should be false")
	}
}

func TestBuildMiddleware_RedirectScheme(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.https-redirect.redirectscheme.scheme":    "https",
		"traefik.http.middlewares.https-redirect.redirectscheme.permanent": "true",
		"traefik.http.middlewares.https-redirect.redirectscheme.port":      "443",
	}
	mw, err := buildMiddleware("redirectscheme", labels, "https-redirect")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.RedirectScheme == nil {
		t.Fatal("expected RedirectScheme to be set")
	}
	if mw.RedirectScheme.Scheme != "https" {
		t.Errorf("Scheme = %q, want %q", mw.RedirectScheme.Scheme, "https")
	}
	if mw.RedirectScheme.Permanent != true {
		t.Error("Permanent should be true")
	}
	if mw.RedirectScheme.Port != "443" {
		t.Errorf("Port = %q, want %q", mw.RedirectScheme.Port, "443")
	}
}

func TestBuildMiddleware_BasicAuth(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-auth.basicauth.users":        "user1:pass1,user2:pass2",
		"traefik.http.middlewares.my-auth.basicauth.realm":        "MyRealm",
		"traefik.http.middlewares.my-auth.basicauth.removeheader": "true",
	}
	mw, err := buildMiddleware("basicauth", labels, "my-auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.BasicAuth == nil {
		t.Fatal("expected BasicAuth to be set")
	}
	if len(mw.BasicAuth.Users) != 2 {
		t.Errorf("Users count = %d, want 2", len(mw.BasicAuth.Users))
	}
	if mw.BasicAuth.Realm != "MyRealm" {
		t.Errorf("Realm = %q, want %q", mw.BasicAuth.Realm, "MyRealm")
	}
	if !mw.BasicAuth.RemoveHeader {
		t.Error("RemoveHeader should be true")
	}
}

func TestBuildMiddleware_Headers(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-headers.headers.framedeny":                       "true",
		"traefik.http.middlewares.my-headers.headers.contenttypenosniff":              "true",
		"traefik.http.middlewares.my-headers.headers.browserxssfilter":                "true",
		"traefik.http.middlewares.my-headers.headers.stsseconds":                      "31536000",
		"traefik.http.middlewares.my-headers.headers.stsincludesubdomains":            "true",
		"traefik.http.middlewares.my-headers.headers.customrequestheaders.x-custom":   "value1",
		"traefik.http.middlewares.my-headers.headers.customresponseheaders.x-powered": "traefik",
		"traefik.http.middlewares.my-headers.headers.accesscontrolallowmethods":       "GET,POST,OPTIONS",
	}
	mw, err := buildMiddleware("headers", labels, "my-headers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.Headers == nil {
		t.Fatal("expected Headers to be set")
	}
	if !mw.Headers.FrameDeny {
		t.Error("FrameDeny should be true")
	}
	if !mw.Headers.ContentTypeNosniff {
		t.Error("ContentTypeNosniff should be true")
	}
	if !mw.Headers.BrowserXSSFilter {
		t.Error("BrowserXSSFilter should be true")
	}
	if mw.Headers.STSSeconds != 31536000 {
		t.Errorf("STSSeconds = %d, want 31536000", mw.Headers.STSSeconds)
	}
	if !mw.Headers.STSIncludeSubdomains {
		t.Error("STSIncludeSubdomains should be true")
	}
	if mw.Headers.CustomRequestHeaders == nil || mw.Headers.CustomRequestHeaders["x-custom"] != "value1" {
		t.Errorf("CustomRequestHeaders[x-custom] = %q, want %q", mw.Headers.CustomRequestHeaders["x-custom"], "value1")
	}
	if mw.Headers.CustomResponseHeaders == nil || mw.Headers.CustomResponseHeaders["x-powered"] != "traefik" {
		t.Errorf("CustomResponseHeaders[x-powered] = %q, want %q", mw.Headers.CustomResponseHeaders["x-powered"], "traefik")
	}
	if len(mw.Headers.AccessControlAllowMethods) != 3 {
		t.Errorf("AccessControlAllowMethods count = %d, want 3", len(mw.Headers.AccessControlAllowMethods))
	}
}

func TestBuildMiddleware_Headers_SecurityRegistry(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.security-headers-registry.headers.sslredirect":                  "true",
		"traefik.http.middlewares.security-headers-registry.headers.framedeny":                    "true",
		"traefik.http.middlewares.security-headers-registry.headers.contenttypenosniff":           "true",
		"traefik.http.middlewares.security-headers-registry.headers.browserxssfilter":             "true",
		"traefik.http.middlewares.security-headers-registry.headers.customframeoptionsvalue":      "SAMEORIGIN",
		"traefik.http.middlewares.security-headers-registry.headers.accesscontrolallowmethods":    "GET,OPTIONS,PUT",
		"traefik.http.middlewares.security-headers-registry.headers.accesscontrolalloworiginlist": "https://registry-ui.t.geekosphere.net",
		"traefik.http.middlewares.security-headers-registry.headers.accesscontrolmaxage":          "100",
		"traefik.http.middlewares.security-headers-registry.headers.addvaryheader":                "true",
	}
	mw, err := buildMiddleware("headers", labels, "security-headers-registry")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.Headers == nil {
		t.Fatal("expected Headers to be set")
	}
	if !mw.Headers.SSLRedirect {
		t.Error("SSLRedirect should be true")
	}
	if !mw.Headers.FrameDeny {
		t.Error("FrameDeny should be true")
	}
	if !mw.Headers.ContentTypeNosniff {
		t.Error("ContentTypeNosniff should be true")
	}
	if !mw.Headers.BrowserXSSFilter {
		t.Error("BrowserXSSFilter should be true")
	}
	if mw.Headers.CustomFrameOptionsValue != "SAMEORIGIN" {
		t.Errorf("CustomFrameOptionsValue = %q, want %q", mw.Headers.CustomFrameOptionsValue, "SAMEORIGIN")
	}
	if len(mw.Headers.AccessControlAllowMethods) != 3 {
		t.Errorf("AccessControlAllowMethods count = %d, want 3", len(mw.Headers.AccessControlAllowMethods))
	}
	if len(mw.Headers.AccessControlAllowOriginList) != 1 || mw.Headers.AccessControlAllowOriginList[0] != "https://registry-ui.t.geekosphere.net" {
		t.Errorf("AccessControlAllowOriginList = %v, want [https://registry-ui.t.geekosphere.net]", mw.Headers.AccessControlAllowOriginList)
	}
	if mw.Headers.AccessControlMaxAge != 100 {
		t.Errorf("AccessControlMaxAge = %d, want 100", mw.Headers.AccessControlMaxAge)
	}
	if !mw.Headers.AddVaryHeader {
		t.Error("AddVaryHeader should be true")
	}
}

func TestBuildMiddleware_Compress(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-compress.compress.excludedcontenttypes": "text/event-stream",
		"traefik.http.middlewares.my-compress.compress.minresponsebodybytes": "1024",
	}
	mw, err := buildMiddleware("compress", labels, "my-compress")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.Compress == nil {
		t.Fatal("expected Compress to be set")
	}
	if len(mw.Compress.ExcludedContentTypes) != 1 || mw.Compress.ExcludedContentTypes[0] != "text/event-stream" {
		t.Errorf("ExcludedContentTypes = %v, want [text/event-stream]", mw.Compress.ExcludedContentTypes)
	}
	if mw.Compress.MinResponseBodyBytes != 1024 {
		t.Errorf("MinResponseBodyBytes = %d, want 1024", mw.Compress.MinResponseBodyBytes)
	}
}

func TestBuildMiddleware_StripPrefix(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-strip.stripprefix.prefixes":   "/api,/v2",
		"traefik.http.middlewares.my-strip.stripprefix.forceslash": "true",
	}
	mw, err := buildMiddleware("stripprefix", labels, "my-strip")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.StripPrefix == nil {
		t.Fatal("expected StripPrefix to be set")
	}
	if len(mw.StripPrefix.Prefixes) != 2 {
		t.Fatalf("Prefixes count = %d, want 2", len(mw.StripPrefix.Prefixes))
	}
	if mw.StripPrefix.Prefixes[0] != "/api" || mw.StripPrefix.Prefixes[1] != "/v2" {
		t.Errorf("Prefixes = %v, want [/api /v2]", mw.StripPrefix.Prefixes)
	}
	if !mw.StripPrefix.ForceSlash {
		t.Error("ForceSlash should be true")
	}
}

func TestBuildMiddleware_Chain(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-chain.chain.middlewares": "auth,compress,headers",
	}
	mw, err := buildMiddleware("chain", labels, "my-chain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.Chain == nil {
		t.Fatal("expected Chain to be set")
	}
	if len(mw.Chain.Middlewares) != 3 {
		t.Fatalf("Middlewares count = %d, want 3", len(mw.Chain.Middlewares))
	}
}

func TestBuildMiddleware_IPAllowList(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-ipallow.ipallowlist.sourcerange":            "10.0.0.0/8,172.16.0.0/12",
		"traefik.http.middlewares.my-ipallow.ipallowlist.ipstrategy.depth":       "2",
		"traefik.http.middlewares.my-ipallow.ipallowlist.ipstrategy.excludedips": "127.0.0.1",
	}
	mw, err := buildMiddleware("ipallowlist", labels, "my-ipallow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.IPAllowList == nil {
		t.Fatal("expected IPAllowList to be set")
	}
	if len(mw.IPAllowList.SourceRange) != 2 {
		t.Errorf("SourceRange count = %d, want 2", len(mw.IPAllowList.SourceRange))
	}
	if mw.IPAllowList.IPStrategy == nil {
		t.Fatal("expected IPStrategy to be set")
	}
	if mw.IPAllowList.IPStrategy.Depth != 2 {
		t.Errorf("Depth = %d, want 2", mw.IPAllowList.IPStrategy.Depth)
	}
}

func TestBuildMiddleware_RateLimit(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-rate.ratelimit.average":                          "100",
		"traefik.http.middlewares.my-rate.ratelimit.burst":                            "50",
		"traefik.http.middlewares.my-rate.ratelimit.period":                           "1m",
		"traefik.http.middlewares.my-rate.ratelimit.sourcecriterion.requestheadername": "X-Real-IP",
	}
	mw, err := buildMiddleware("ratelimit", labels, "my-rate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.RateLimit == nil {
		t.Fatal("expected RateLimit to be set")
	}
	if mw.RateLimit.Average != 100 {
		t.Errorf("Average = %d, want 100", mw.RateLimit.Average)
	}
	if mw.RateLimit.Burst != 50 {
		t.Errorf("Burst = %d, want 50", mw.RateLimit.Burst)
	}
	if mw.RateLimit.Period != "1m" {
		t.Errorf("Period = %q, want %q", mw.RateLimit.Period, "1m")
	}
	if mw.RateLimit.SourceCriterion == nil {
		t.Fatal("expected SourceCriterion to be set")
	}
	if mw.RateLimit.SourceCriterion.RequestHeaderName != "X-Real-IP" {
		t.Errorf("RequestHeaderName = %q, want %q", mw.RateLimit.SourceCriterion.RequestHeaderName, "X-Real-IP")
	}
}

func TestBuildMiddleware_UnknownType(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.bad.nonexistent.foo": "bar",
	}
	_, err := buildMiddleware("nonexistent", labels, "bad")
	if err == nil {
		t.Fatal("expected error for unknown middleware type")
	}
}

func TestGetLabelsByPrefix(t *testing.T) {
	config := map[string]string{
		"traefik.http.middlewares.test.redirectregex.regex":       "^.*$",
		"traefik.http.middlewares.test.redirectregex.replacement": "http://example.com",
		"traefik.http.routers.test.rule":                         "Host(`test`)",
	}
	result := getLabelsByPrefix(config, "traefik.http.middlewares.test.redirectregex.")
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result["regex"] != "^.*$" {
		t.Errorf("regex = %q, want %q", result["regex"], "^.*$")
	}
	if result["replacement"] != "http://example.com" {
		t.Errorf("replacement = %q, want %q", result["replacement"], "http://example.com")
	}
}

func TestSplitCommaSeparated(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"a , b , c", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"", []string{}},
		{" , , ", []string{}},
	}
	for _, tt := range tests {
		result := splitCommaSeparated(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitCommaSeparated(%q) = %v (len %d), want %v (len %d)", tt.input, result, len(result), tt.expected, len(tt.expected))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("splitCommaSeparated(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestGenerateConfiguration_WithMiddlewares(t *testing.T) {
	servicesMap := map[string][]internal.Service{
		"node1": {
			{
				ID:   100,
				Name: "speedtest",
				IPs:  []internal.IP{{Address: "192.168.4.165", AddressType: "ipv4"}},
				Config: map[string]string{
					"traefik.enable":                                                        "true",
					"traefik.http.routers.speedtest.rule":                                   "Host(`speedtest.example.com`)",
					"traefik.http.routers.speedtest.middlewares":                             "speedtest-redirect",
					"traefik.http.services.speedtest.loadbalancer.server.port":               "3000",
					"traefik.http.middlewares.speedtest-redirect.redirectregex.regex":         "^.*$",
					"traefik.http.middlewares.speedtest-redirect.redirectregex.replacement":   "http://192.168.4.165:3000",
					"traefik.http.middlewares.speedtest-redirect.redirectregex.permanent":     "false",
				},
			},
		},
	}

	config := generateConfiguration(servicesMap)

	if len(config.HTTP.Middlewares) != 1 {
		t.Fatalf("expected 1 middleware, got %d", len(config.HTTP.Middlewares))
	}

	mw, ok := config.HTTP.Middlewares["speedtest-redirect"]
	if !ok {
		t.Fatal("expected middleware 'speedtest-redirect' to exist")
	}
	if mw.RedirectRegex == nil {
		t.Fatal("expected RedirectRegex to be set")
	}
	if mw.RedirectRegex.Regex != "^.*$" {
		t.Errorf("Regex = %q, want %q", mw.RedirectRegex.Regex, "^.*$")
	}
	if mw.RedirectRegex.Replacement != "http://192.168.4.165:3000" {
		t.Errorf("Replacement = %q, want %q", mw.RedirectRegex.Replacement, "http://192.168.4.165:3000")
	}

	router, ok := config.HTTP.Routers["speedtest"]
	if !ok {
		t.Fatal("expected router 'speedtest' to exist")
	}
	if len(router.Middlewares) != 1 || router.Middlewares[0] != "speedtest-redirect" {
		t.Errorf("router middlewares = %v, want [speedtest-redirect]", router.Middlewares)
	}
}

func TestGenerateConfiguration_MultipleMiddlewares(t *testing.T) {
	servicesMap := map[string][]internal.Service{
		"node1": {
			{
				ID:   200,
				Name: "webapp",
				IPs:  []internal.IP{{Address: "10.0.0.5", AddressType: "ipv4"}},
				Config: map[string]string{
					"traefik.enable":                                                       "true",
					"traefik.http.routers.webapp.rule":                                     "Host(`webapp.example.com`)",
					"traefik.http.routers.webapp.middlewares":                               "my-compress,my-headers",
					"traefik.http.services.webapp.loadbalancer.server.port":                 "8080",
					"traefik.http.middlewares.my-compress.compress.minresponsebodybytes":    "512",
					"traefik.http.middlewares.my-headers.headers.framedeny":                 "true",
					"traefik.http.middlewares.my-headers.headers.customrequestheaders.x-app": "test",
				},
			},
		},
	}

	config := generateConfiguration(servicesMap)

	if len(config.HTTP.Middlewares) != 2 {
		t.Fatalf("expected 2 middlewares, got %d", len(config.HTTP.Middlewares))
	}

	compress, ok := config.HTTP.Middlewares["my-compress"]
	if !ok {
		t.Fatal("expected middleware 'my-compress' to exist")
	}
	if compress.Compress == nil {
		t.Fatal("expected Compress to be set")
	}
	if compress.Compress.MinResponseBodyBytes != 512 {
		t.Errorf("MinResponseBodyBytes = %d, want 512", compress.Compress.MinResponseBodyBytes)
	}

	headers, ok := config.HTTP.Middlewares["my-headers"]
	if !ok {
		t.Fatal("expected middleware 'my-headers' to exist")
	}
	if headers.Headers == nil {
		t.Fatal("expected Headers to be set")
	}
	if !headers.Headers.FrameDeny {
		t.Error("FrameDeny should be true")
	}
	if headers.Headers.CustomRequestHeaders["x-app"] != "test" {
		t.Errorf("CustomRequestHeaders[x-app] = %q, want %q", headers.Headers.CustomRequestHeaders["x-app"], "test")
	}
}

func TestBuildMiddleware_ForwardAuth(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-fwd.forwardauth.address":                "http://auth.example.com",
		"traefik.http.middlewares.my-fwd.forwardauth.trustforwardheader":     "true",
		"traefik.http.middlewares.my-fwd.forwardauth.authresponseheaders":    "X-Auth-User,X-Auth-Role",
		"traefik.http.middlewares.my-fwd.forwardauth.tls.insecureskipverify": "true",
	}
	mw, err := buildMiddleware("forwardauth", labels, "my-fwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.ForwardAuth == nil {
		t.Fatal("expected ForwardAuth to be set")
	}
	if mw.ForwardAuth.Address != "http://auth.example.com" {
		t.Errorf("Address = %q, want %q", mw.ForwardAuth.Address, "http://auth.example.com")
	}
	if !mw.ForwardAuth.TrustForwardHeader {
		t.Error("TrustForwardHeader should be true")
	}
	if len(mw.ForwardAuth.AuthResponseHeaders) != 2 {
		t.Errorf("AuthResponseHeaders count = %d, want 2", len(mw.ForwardAuth.AuthResponseHeaders))
	}
	if mw.ForwardAuth.TLS == nil {
		t.Fatal("expected TLS to be set")
	}
	if !mw.ForwardAuth.TLS.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}

func TestBuildMiddleware_Retry(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-retry.retry.attempts":        "3",
		"traefik.http.middlewares.my-retry.retry.initialinterval": "100ms",
	}
	mw, err := buildMiddleware("retry", labels, "my-retry")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.Retry == nil {
		t.Fatal("expected Retry to be set")
	}
	if mw.Retry.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", mw.Retry.Attempts)
	}
	if mw.Retry.InitialInterval != "100ms" {
		t.Errorf("InitialInterval = %q, want %q", mw.Retry.InitialInterval, "100ms")
	}
}

func TestBuildMiddleware_AddPrefix(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-prefix.addprefix.prefix": "/api",
	}
	mw, err := buildMiddleware("addprefix", labels, "my-prefix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.AddPrefix == nil {
		t.Fatal("expected AddPrefix to be set")
	}
	if mw.AddPrefix.Prefix != "/api" {
		t.Errorf("Prefix = %q, want %q", mw.AddPrefix.Prefix, "/api")
	}
}

func TestBuildMiddleware_Buffering(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-buf.buffering.maxrequestbodybytes":  "1048576",
		"traefik.http.middlewares.my-buf.buffering.maxresponsebodybytes": "2097152",
		"traefik.http.middlewares.my-buf.buffering.retryexpression":      "IsNetworkError()",
	}
	mw, err := buildMiddleware("buffering", labels, "my-buf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.Buffering == nil {
		t.Fatal("expected Buffering to be set")
	}
	if mw.Buffering.MaxRequestBodyBytes != 1048576 {
		t.Errorf("MaxRequestBodyBytes = %d, want 1048576", mw.Buffering.MaxRequestBodyBytes)
	}
	if mw.Buffering.MaxResponseBodyBytes != 2097152 {
		t.Errorf("MaxResponseBodyBytes = %d, want 2097152", mw.Buffering.MaxResponseBodyBytes)
	}
	if mw.Buffering.RetryExpression != "IsNetworkError()" {
		t.Errorf("RetryExpression = %q, want %q", mw.Buffering.RetryExpression, "IsNetworkError()")
	}
}

func TestBuildMiddleware_CircuitBreaker(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-cb.circuitbreaker.expression": "NetworkErrorRatio() > 0.5",
	}
	mw, err := buildMiddleware("circuitbreaker", labels, "my-cb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.CircuitBreaker == nil {
		t.Fatal("expected CircuitBreaker to be set")
	}
	if mw.CircuitBreaker.Expression != "NetworkErrorRatio() > 0.5" {
		t.Errorf("Expression = %q, want %q", mw.CircuitBreaker.Expression, "NetworkErrorRatio() > 0.5")
	}
}

func TestBuildMiddleware_ErrorPage(t *testing.T) {
	labels := map[string]string{
		"traefik.http.middlewares.my-errors.errors.status":  "500-599",
		"traefik.http.middlewares.my-errors.errors.service": "error-svc",
		"traefik.http.middlewares.my-errors.errors.query":   "/{status}.html",
	}
	mw, err := buildMiddleware("errors", labels, "my-errors")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.Errors == nil {
		t.Fatal("expected Errors to be set")
	}
	if mw.Errors.Service != "error-svc" {
		t.Errorf("Service = %q, want %q", mw.Errors.Service, "error-svc")
	}
	if mw.Errors.Query != "/{status}.html" {
		t.Errorf("Query = %q, want %q", mw.Errors.Query, "/{status}.html")
	}
	if len(mw.Errors.Status) != 1 || mw.Errors.Status[0] != "500-599" {
		t.Errorf("Status = %v, want [500-599]", mw.Errors.Status)
	}
}
