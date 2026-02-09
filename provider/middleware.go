package provider

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/traefik/genconf/dynamic"
	"github.com/traefik/genconf/dynamic/types"
)

// getLabelsByPrefix returns all labels matching the given prefix with the prefix stripped.
func getLabelsByPrefix(config map[string]string, prefix string) map[string]string {
	result := make(map[string]string)
	for k, v := range config {
		if strings.HasPrefix(k, prefix) {
			result[strings.TrimPrefix(k, prefix)] = v
		}
	}
	return result
}

// splitCommaSeparated splits a comma-separated string and trims whitespace from each element.
func splitCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// stringToInt64 converts a string to int64.
func stringToInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// buildMiddleware dispatches to type-specific builders based on the middleware type name.
func buildMiddleware(mwType string, allLabels map[string]string, mwName string) (*dynamic.Middleware, error) {
	prefix := fmt.Sprintf("traefik.http.middlewares.%s.%s.", mwName, mwType)
	fields := getLabelsByPrefix(allLabels, prefix)

	switch strings.ToLower(mwType) {
	case "addprefix":
		return buildAddPrefix(fields)
	case "stripprefix":
		return buildStripPrefix(fields)
	case "stripprefixregex":
		return buildStripPrefixRegex(fields)
	case "replacepath":
		return buildReplacePath(fields)
	case "replacepathregex":
		return buildReplacePathRegex(fields)
	case "redirectregex":
		return buildRedirectRegex(fields)
	case "redirectscheme":
		return buildRedirectScheme(fields)
	case "chain":
		return buildChain(fields)
	case "basicauth":
		return buildBasicAuth(fields)
	case "digestauth":
		return buildDigestAuth(fields)
	case "forwardauth":
		return buildForwardAuth(fields)
	case "headers":
		return buildHeaders(fields)
	case "ipallowlist":
		return buildIPAllowList(fields)
	case "ipwhitelist":
		return buildIPWhiteList(fields)
	case "ratelimit":
		return buildRateLimit(fields)
	case "inflightreq":
		return buildInFlightReq(fields)
	case "buffering":
		return buildBuffering(fields)
	case "circuitbreaker":
		return buildCircuitBreaker(fields)
	case "compress":
		return buildCompress(fields)
	case "contenttype":
		return buildContentType(fields)
	case "errors":
		return buildErrorPage(fields)
	case "passtlsclientcert":
		return buildPassTLSClientCert(fields)
	case "retry":
		return buildRetry(fields)
	default:
		return nil, fmt.Errorf("unknown middleware type: %s", mwType)
	}
}

func buildAddPrefix(fields map[string]string) (*dynamic.Middleware, error) {
	return &dynamic.Middleware{
		AddPrefix: &dynamic.AddPrefix{
			Prefix: fields["prefix"],
		},
	}, nil
}

func buildStripPrefix(fields map[string]string) (*dynamic.Middleware, error) {
	sp := &dynamic.StripPrefix{}
	if v, ok := fields["prefixes"]; ok {
		sp.Prefixes = splitCommaSeparated(v)
	}
	if v, ok := fields["forceslash"]; ok {
		if b, err := stringToBool(v); err == nil {
			sp.ForceSlash = b
		}
	}
	return &dynamic.Middleware{StripPrefix: sp}, nil
}

func buildStripPrefixRegex(fields map[string]string) (*dynamic.Middleware, error) {
	spr := &dynamic.StripPrefixRegex{}
	if v, ok := fields["regex"]; ok {
		spr.Regex = splitCommaSeparated(v)
	}
	return &dynamic.Middleware{StripPrefixRegex: spr}, nil
}

func buildReplacePath(fields map[string]string) (*dynamic.Middleware, error) {
	return &dynamic.Middleware{
		ReplacePath: &dynamic.ReplacePath{
			Path: fields["path"],
		},
	}, nil
}

func buildReplacePathRegex(fields map[string]string) (*dynamic.Middleware, error) {
	return &dynamic.Middleware{
		ReplacePathRegex: &dynamic.ReplacePathRegex{
			Regex:       fields["regex"],
			Replacement: fields["replacement"],
		},
	}, nil
}

func buildRedirectRegex(fields map[string]string) (*dynamic.Middleware, error) {
	rr := &dynamic.RedirectRegex{
		Regex:       fields["regex"],
		Replacement: fields["replacement"],
	}
	if v, ok := fields["permanent"]; ok {
		if b, err := stringToBool(v); err == nil {
			rr.Permanent = b
		}
	}
	return &dynamic.Middleware{RedirectRegex: rr}, nil
}

func buildRedirectScheme(fields map[string]string) (*dynamic.Middleware, error) {
	rs := &dynamic.RedirectScheme{
		Scheme: fields["scheme"],
		Port:   fields["port"],
	}
	if v, ok := fields["permanent"]; ok {
		if b, err := stringToBool(v); err == nil {
			rs.Permanent = b
		}
	}
	return &dynamic.Middleware{RedirectScheme: rs}, nil
}

func buildChain(fields map[string]string) (*dynamic.Middleware, error) {
	c := &dynamic.Chain{}
	if v, ok := fields["middlewares"]; ok {
		c.Middlewares = splitCommaSeparated(v)
	}
	return &dynamic.Middleware{Chain: c}, nil
}

func buildBasicAuth(fields map[string]string) (*dynamic.Middleware, error) {
	ba := &dynamic.BasicAuth{
		UsersFile:   fields["usersfile"],
		Realm:       fields["realm"],
		HeaderField: fields["headerfield"],
	}
	if v, ok := fields["users"]; ok {
		ba.Users = splitCommaSeparated(v)
	}
	if v, ok := fields["removeheader"]; ok {
		if b, err := stringToBool(v); err == nil {
			ba.RemoveHeader = b
		}
	}
	return &dynamic.Middleware{BasicAuth: ba}, nil
}

func buildDigestAuth(fields map[string]string) (*dynamic.Middleware, error) {
	da := &dynamic.DigestAuth{
		UsersFile:   fields["usersfile"],
		Realm:       fields["realm"],
		HeaderField: fields["headerfield"],
	}
	if v, ok := fields["users"]; ok {
		da.Users = splitCommaSeparated(v)
	}
	if v, ok := fields["removeheader"]; ok {
		if b, err := stringToBool(v); err == nil {
			da.RemoveHeader = b
		}
	}
	return &dynamic.Middleware{DigestAuth: da}, nil
}

func buildForwardAuth(fields map[string]string) (*dynamic.Middleware, error) {
	fa := &dynamic.ForwardAuth{
		Address:                  fields["address"],
		AuthResponseHeadersRegex: fields["authresponseheadersregex"],
	}
	if v, ok := fields["trustforwardheader"]; ok {
		if b, err := stringToBool(v); err == nil {
			fa.TrustForwardHeader = b
		}
	}
	if v, ok := fields["authresponseheaders"]; ok {
		fa.AuthResponseHeaders = splitCommaSeparated(v)
	}
	if v, ok := fields["authrequestheaders"]; ok {
		fa.AuthRequestHeaders = splitCommaSeparated(v)
	}
	tls := buildClientTLS(fields)
	if tls != nil {
		fa.TLS = tls
	}
	return &dynamic.Middleware{ForwardAuth: fa}, nil
}

func buildClientTLS(fields map[string]string) *types.ClientTLS {
	ca := fields["tls.ca"]
	cert := fields["tls.cert"]
	key := fields["tls.key"]
	insecureSkipVerify := fields["tls.insecureskipverify"]

	if ca == "" && cert == "" && key == "" && insecureSkipVerify == "" {
		return nil
	}

	t := &types.ClientTLS{
		CA:   ca,
		Cert: cert,
		Key:  key,
	}
	if insecureSkipVerify != "" {
		if b, err := stringToBool(insecureSkipVerify); err == nil {
			t.InsecureSkipVerify = b
		}
	}
	return t
}

func buildIPStrategy(fields map[string]string, prefix string) *dynamic.IPStrategy {
	depthStr := fields[prefix+"ipstrategy.depth"]
	excludedIPs := fields[prefix+"ipstrategy.excludedips"]

	if depthStr == "" && excludedIPs == "" {
		return nil
	}

	ips := &dynamic.IPStrategy{}
	if depthStr != "" {
		if d, err := stringToInt(depthStr); err == nil {
			ips.Depth = d
		}
	}
	if excludedIPs != "" {
		ips.ExcludedIPs = splitCommaSeparated(excludedIPs)
	}
	return ips
}

func buildSourceCriterion(fields map[string]string, prefix string) *dynamic.SourceCriterion {
	reqHeaderName := fields[prefix+"sourcecriterion.requestheadername"]
	reqHost := fields[prefix+"sourcecriterion.requesthost"]
	ipStrategy := buildIPStrategy(fields, prefix+"sourcecriterion.")

	if reqHeaderName == "" && reqHost == "" && ipStrategy == nil {
		return nil
	}

	sc := &dynamic.SourceCriterion{
		RequestHeaderName: reqHeaderName,
		IPStrategy:        ipStrategy,
	}
	if reqHost != "" {
		if b, err := stringToBool(reqHost); err == nil {
			sc.RequestHost = b
		}
	}
	return sc
}

func buildIPAllowList(fields map[string]string) (*dynamic.Middleware, error) {
	al := &dynamic.IPAllowList{}
	if v, ok := fields["sourcerange"]; ok {
		al.SourceRange = splitCommaSeparated(v)
	}
	al.IPStrategy = buildIPStrategy(fields, "")
	return &dynamic.Middleware{IPAllowList: al}, nil
}

func buildIPWhiteList(fields map[string]string) (*dynamic.Middleware, error) {
	wl := &dynamic.IPWhiteList{}
	if v, ok := fields["sourcerange"]; ok {
		wl.SourceRange = splitCommaSeparated(v)
	}
	wl.IPStrategy = buildIPStrategy(fields, "")
	return &dynamic.Middleware{IPWhiteList: wl}, nil
}

func buildRateLimit(fields map[string]string) (*dynamic.Middleware, error) {
	rl := &dynamic.RateLimit{
		Period: fields["period"],
	}
	if v, ok := fields["average"]; ok {
		if n, err := stringToInt64(v); err == nil {
			rl.Average = n
		}
	}
	if v, ok := fields["burst"]; ok {
		if n, err := stringToInt64(v); err == nil {
			rl.Burst = n
		}
	}
	rl.SourceCriterion = buildSourceCriterion(fields, "")
	return &dynamic.Middleware{RateLimit: rl}, nil
}

func buildInFlightReq(fields map[string]string) (*dynamic.Middleware, error) {
	ifr := &dynamic.InFlightReq{}
	if v, ok := fields["amount"]; ok {
		if n, err := stringToInt64(v); err == nil {
			ifr.Amount = n
		}
	}
	ifr.SourceCriterion = buildSourceCriterion(fields, "")
	return &dynamic.Middleware{InFlightReq: ifr}, nil
}

func buildBuffering(fields map[string]string) (*dynamic.Middleware, error) {
	b := &dynamic.Buffering{
		RetryExpression: fields["retryexpression"],
	}
	if v, ok := fields["maxrequestbodybytes"]; ok {
		if n, err := stringToInt64(v); err == nil {
			b.MaxRequestBodyBytes = n
		}
	}
	if v, ok := fields["memrequestbodybytes"]; ok {
		if n, err := stringToInt64(v); err == nil {
			b.MemRequestBodyBytes = n
		}
	}
	if v, ok := fields["maxresponsebodybytes"]; ok {
		if n, err := stringToInt64(v); err == nil {
			b.MaxResponseBodyBytes = n
		}
	}
	if v, ok := fields["memresponsebodybytes"]; ok {
		if n, err := stringToInt64(v); err == nil {
			b.MemResponseBodyBytes = n
		}
	}
	return &dynamic.Middleware{Buffering: b}, nil
}

func buildCircuitBreaker(fields map[string]string) (*dynamic.Middleware, error) {
	return &dynamic.Middleware{
		CircuitBreaker: &dynamic.CircuitBreaker{
			Expression:       fields["expression"],
			CheckPeriod:      fields["checkperiod"],
			FallbackDuration: fields["fallbackduration"],
			RecoveryDuration: fields["recoveryduration"],
		},
	}, nil
}

func buildCompress(fields map[string]string) (*dynamic.Middleware, error) {
	c := &dynamic.Compress{}
	if v, ok := fields["excludedcontenttypes"]; ok {
		c.ExcludedContentTypes = splitCommaSeparated(v)
	}
	if v, ok := fields["minresponsebodybytes"]; ok {
		if n, err := stringToInt(v); err == nil {
			c.MinResponseBodyBytes = n
		}
	}
	return &dynamic.Middleware{Compress: c}, nil
}

func buildContentType(fields map[string]string) (*dynamic.Middleware, error) {
	ct := &dynamic.ContentType{}
	if v, ok := fields["autodetect"]; ok {
		if b, err := stringToBool(v); err == nil {
			ct.AutoDetect = b
		}
	}
	return &dynamic.Middleware{ContentType: ct}, nil
}

func buildErrorPage(fields map[string]string) (*dynamic.Middleware, error) {
	ep := &dynamic.ErrorPage{
		Service: fields["service"],
		Query:   fields["query"],
	}
	if v, ok := fields["status"]; ok {
		ep.Status = splitCommaSeparated(v)
	}
	return &dynamic.Middleware{Errors: ep}, nil
}

func buildPassTLSClientCert(fields map[string]string) (*dynamic.Middleware, error) {
	p := &dynamic.PassTLSClientCert{}
	if v, ok := fields["pem"]; ok {
		if b, err := stringToBool(v); err == nil {
			p.PEM = b
		}
	}
	info := buildTLSClientCertInfo(fields)
	if info != nil {
		p.Info = info
	}
	return &dynamic.Middleware{PassTLSClientCert: p}, nil
}

func buildTLSClientCertInfo(fields map[string]string) *dynamic.TLSClientCertificateInfo {
	info := &dynamic.TLSClientCertificateInfo{}
	hasField := false

	boolFields := map[string]*bool{
		"info.notafter":     &info.NotAfter,
		"info.notbefore":    &info.NotBefore,
		"info.sans":         &info.Sans,
		"info.serialnumber": &info.SerialNumber,
	}
	for key, target := range boolFields {
		if v, ok := fields[key]; ok {
			if b, err := stringToBool(v); err == nil {
				*target = b
				hasField = true
			}
		}
	}

	subject := buildTLSSubjectDN(fields)
	if subject != nil {
		info.Subject = subject
		hasField = true
	}

	issuer := buildTLSIssuerDN(fields)
	if issuer != nil {
		info.Issuer = issuer
		hasField = true
	}

	if !hasField {
		return nil
	}
	return info
}

func buildTLSSubjectDN(fields map[string]string) *dynamic.TLSClientCertificateSubjectDNInfo {
	s := &dynamic.TLSClientCertificateSubjectDNInfo{}
	hasField := false

	boolFields := map[string]*bool{
		"info.subject.country":            &s.Country,
		"info.subject.province":           &s.Province,
		"info.subject.locality":           &s.Locality,
		"info.subject.organization":       &s.Organization,
		"info.subject.organizationalunit": &s.OrganizationalUnit,
		"info.subject.commonname":         &s.CommonName,
		"info.subject.serialnumber":       &s.SerialNumber,
		"info.subject.domaincomponent":    &s.DomainComponent,
	}
	for key, target := range boolFields {
		if v, ok := fields[key]; ok {
			if b, err := stringToBool(v); err == nil {
				*target = b
				hasField = true
			}
		}
	}

	if !hasField {
		return nil
	}
	return s
}

func buildTLSIssuerDN(fields map[string]string) *dynamic.TLSClientCertificateIssuerDNInfo {
	i := &dynamic.TLSClientCertificateIssuerDNInfo{}
	hasField := false

	boolFields := map[string]*bool{
		"info.issuer.country":         &i.Country,
		"info.issuer.province":        &i.Province,
		"info.issuer.locality":        &i.Locality,
		"info.issuer.organization":    &i.Organization,
		"info.issuer.commonname":      &i.CommonName,
		"info.issuer.serialnumber":    &i.SerialNumber,
		"info.issuer.domaincomponent": &i.DomainComponent,
	}
	for key, target := range boolFields {
		if v, ok := fields[key]; ok {
			if b, err := stringToBool(v); err == nil {
				*target = b
				hasField = true
			}
		}
	}

	if !hasField {
		return nil
	}
	return i
}

func buildRetry(fields map[string]string) (*dynamic.Middleware, error) {
	r := &dynamic.Retry{
		InitialInterval: fields["initialinterval"],
	}
	if v, ok := fields["attempts"]; ok {
		if n, err := stringToInt(v); err == nil {
			r.Attempts = n
		}
	}
	return &dynamic.Middleware{Retry: r}, nil
}

func buildHeaders(fields map[string]string) (*dynamic.Middleware, error) {
	h := &dynamic.Headers{}

	// String fields
	stringFields := map[string]*string{
		"sslhost":                 &h.SSLHost,
		"customframeoptionsvalue": &h.CustomFrameOptionsValue,
		"custombrowserxssvalue":   &h.CustomBrowserXSSValue,
		"contentsecuritypolicy":   &h.ContentSecurityPolicy,
		"publickey":               &h.PublicKey,
		"referrerpolicy":          &h.ReferrerPolicy,
		"featurepolicy":           &h.FeaturePolicy,
		"permissionspolicy":       &h.PermissionsPolicy,
	}
	for key, target := range stringFields {
		if v, ok := fields[key]; ok {
			*target = v
		}
	}

	// Bool fields
	boolFields := map[string]*bool{
		"accesscontrolallowcredentials": &h.AccessControlAllowCredentials,
		"addvaryheader":                 &h.AddVaryHeader,
		"sslredirect":                   &h.SSLRedirect,
		"ssltemporaryredirect":          &h.SSLTemporaryRedirect,
		"sslforcehost":                  &h.SSLForceHost,
		"stsincludesubdomains":          &h.STSIncludeSubdomains,
		"stspreload":                    &h.STSPreload,
		"forcestsheader":                &h.ForceSTSHeader,
		"framedeny":                     &h.FrameDeny,
		"contenttypenosniff":            &h.ContentTypeNosniff,
		"browserxssfilter":              &h.BrowserXSSFilter,
		"isdevelopment":                 &h.IsDevelopment,
	}
	for key, target := range boolFields {
		if v, ok := fields[key]; ok {
			if b, err := stringToBool(v); err == nil {
				*target = b
			}
		}
	}

	// Int64 fields
	if v, ok := fields["accesscontrolmaxage"]; ok {
		if n, err := stringToInt64(v); err == nil {
			h.AccessControlMaxAge = n
		}
	}
	if v, ok := fields["stsseconds"]; ok {
		if n, err := stringToInt64(v); err == nil {
			h.STSSeconds = n
		}
	}

	// Slice fields
	sliceFields := map[string]*[]string{
		"accesscontrolallowheaders":         &h.AccessControlAllowHeaders,
		"accesscontrolallowmethods":         &h.AccessControlAllowMethods,
		"accesscontrolalloworiginlist":      &h.AccessControlAllowOriginList,
		"accesscontrolalloworiginlistregex": &h.AccessControlAllowOriginListRegex,
		"accesscontrolexposeheaders":        &h.AccessControlExposeHeaders,
		"allowedhosts":                      &h.AllowedHosts,
		"hostsproxyheaders":                 &h.HostsProxyHeaders,
	}
	for key, target := range sliceFields {
		if v, ok := fields[key]; ok {
			*target = splitCommaSeparated(v)
		}
	}

	// Map fields: customrequestheaders.*, customresponseheaders.*, sslproxyheaders.*
	h.CustomRequestHeaders = buildMapFromSubKeys(fields, "customrequestheaders.")
	h.CustomResponseHeaders = buildMapFromSubKeys(fields, "customresponseheaders.")
	h.SSLProxyHeaders = buildMapFromSubKeys(fields, "sslproxyheaders.")

	return &dynamic.Middleware{Headers: h}, nil
}

// buildMapFromSubKeys extracts sub-keys with a given prefix into a map.
func buildMapFromSubKeys(fields map[string]string, prefix string) map[string]string {
	result := make(map[string]string)
	for k, v := range fields {
		if strings.HasPrefix(k, prefix) {
			subKey := strings.TrimPrefix(k, prefix)
			if subKey != "" {
				result[subKey] = v
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
