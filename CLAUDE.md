# Traefik Proxmox Provider

A Traefik plugin that uses a Proxmox VE cluster as a provider for dynamic configuration.

## Project Structure

- `provider/provider.go` — Main plugin logic: polling, service scanning, Traefik dynamic config generation
- `provider/middleware.go` — Middleware builder functions for all 23 Traefik HTTP middleware types
- `provider/middleware_test.go` — Tests for middleware builders and integration
- `internal/client.go` — Proxmox API HTTP client
- `internal/models.go` — Data types (Service, ParsedConfig, IP, etc.)

## Branches

- **`main`** — Stable base, uses `log.Printf` for logging, Go 1.19
- **`feat/middleware-support`** — Middleware creation support, branched from `main`, uses `log.Printf`
- **`feat/slog-structured-logging`** — Migrated logging to `log/slog`, also has middleware support with slog logging

## Key Constraints

- **Yaegi runtime**: This plugin runs inside Traefik's Yaegi Go interpreter. Only standard library packages are available — no third-party logging libraries (zerolog, logrus, zap).
- **No `unsafe` package**: Traefik disables `unsafe` for plugins. This doesn't affect stdlib packages (loaded pre-compiled), but blocks third-party libs that import `unsafe`.
- **Logging**: `main` and `feat/middleware-support` use `log.Printf`. `feat/slog-structured-logging` uses `log/slog` (requires Go 1.21+).

## Middleware Support

Middlewares are defined via Proxmox VM description labels:
```
traefik.http.middlewares.<name>.<type>.<field>=<value>
```

The `generateConfiguration()` function scans for `traefik.http.middlewares.*` prefixes, extracts middleware name and type, then dispatches to type-specific builder functions in `middleware.go`.

Supported types: AddPrefix, StripPrefix, StripPrefixRegex, ReplacePath, ReplacePathRegex, RedirectRegex, RedirectScheme, Chain, BasicAuth, DigestAuth, ForwardAuth, Headers, IPAllowList, IPWhiteList, RateLimit, InFlightReq, Buffering, CircuitBreaker, Compress, ContentType, Errors, PassTLSClientCert, Retry.

## Build & Test

```bash
make test        # Run unit tests
make lint        # Run golangci-lint
make yaegi_test  # Verify Yaegi interpreter compatibility
make vendor      # Update vendored dependencies
```

Note: `make test` exits with code 2 due to missing `covdata` tool — all tests still pass. Check `go test -v ./...` output for actual results.
