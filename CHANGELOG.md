# Changelog

## [Unreleased]

### Breaking

- **Router and service names emitted by this provider are now suffixed with `-<vmid>`.**
  Previously, two guests that both used the same router/service name (e.g.
  `traefik.http.routers.app.rule=…`) silently overwrote each other in the
  generated config. Names are now namespaced by Proxmox VMID — `app` becomes
  `app-101`. **Migration:** any external Traefik config that referenced these
  names by their bare form (for example, a middleware chain in the file
  provider pointing at `app@plugin-traefik-proxmox-provider`) must be updated
  to the suffixed name. Routing behavior is otherwise unchanged.

### Fixed

- **Router priority no longer hardcoded.** Routers were emitted with
  `Priority: 1`, which defeats Traefik's rule-length-based default ordering
  and causes nondeterministic match order when multiple rules tied at 1.
  Priority is now left at 0 (omitted) unless an explicit
  `traefik.http.routers.<n>.priority` label is set.
- **`traefik.enable=1` now works.** The boolean check matched only the literal
  string `"true"`. It now uses `strconv.ParseBool` to match Traefik's own
  Docker provider semantics (`1`, `t`, `T`, `true`, `True`, `TRUE` are all
  truthy; `yes`/`on` are intentionally not accepted).
- **Multi-IP guests are now load-balanced.** Previously only the first
  discovered IP was added as a backend; the remaining IPs were dropped on the
  floor. Each guest-agent IP now becomes its own `Server` entry in the load
  balancer, sorted lexicographically for stable output across polls.
- **Router→service binding is now deterministic.** `mapKeysToSlice` returned
  map keys in random iteration order, so the default-target service for a
  router with no explicit `…service=` label could flap between polls.
- **Offline Proxmox nodes are now skipped.** The plugin now reads `status`
  from `/nodes` and skips any node not reported as `online`, instead of
  failing through to a noisy per-cycle scan error.
- **Better diagnostics for PVE 9 token-role errors.** Proxmox VE 9 removed
  the `VM.Monitor` privilege and replaced it with `VM.GuestAgent.Audit`.
  Tokens carrying the old role now get a one-shot warning per VM pointing at
  the README's "Proxmox API Token Setup" section instead of an opaque
  generic 403.
- **Router/service names containing dots are now rejected.** Previously the
  parser silently truncated `traefik.http.routers.my.app.rule` to a router
  named `my`. Such labels now log a warning and are skipped.
- **TLS-domain regex no longer recompiled on every poll.** Hoisted to a
  package-level `var`.

### Added

- Typed sentinel errors in `internal` (`ErrUnauthorized`, `ErrForbidden`,
  `ErrBadRequest`, `ErrServerError`) for callers that need to disambiguate
  Proxmox API failure modes via `errors.Is`.
- Substantially expanded test coverage for `generateConfiguration`,
  `applyRouterOptions`, `extractLabelName`, `mapKeysToSlice`,
  `isBoolLabelEnabled`, and the multi-IP fanout in `getServiceURLs`.

## [v0.7.0] - 2024-03-28

### Added

- Support for all Traefik label options in routers and services
- Proper handling of entrypoints, including singular and plural forms
- Full TLS configuration support including certResolver, options, and domains
- Middleware integration with comma-separated lists
- Support for HTTPS service URLs with proper protocol and default port detection
- Health check configuration for load balancers
- Sticky sessions support with cookie configuration 
- Response forwarding options
- Advanced router options (priority, middlewares)

### Fixed

- Router entrypoint configuration now properly respected
- TLS certificate resolver settings correctly applied
- Middleware configurations properly passed to routers
- Service URLs now use the correct protocol (http/https)
- Default ports updated based on protocol (80 for HTTP, 443 for HTTPS)

### Changed

- Configuration generation completely refactored for label compatibility
- More modular code structure with dedicated functions for router and service options
- Improved logging with clearer messages about configuration creation

## [v0.6.0] - 2024-03-27

### Added

- Respect for original router and service names in labels
- Improved port and URL detection for services
- Support for direct URL overrides via `loadbalancer.server.url` labels
- Better error handling and feedback when IPs can't be found

### Fixed

- Container labels with router/service names like `grafana` are now properly respected
- Port settings for named services are correctly applied
- Better handling for linking routers to the right services

### Changed

- Service discovery now prioritizes explicitly named routers and services
- Default naming (container-id based) only used as fallback when no explicit names found

## [v0.5.0] - 2024-03-27

### Added

- Support for `key=value` format in VM/container descriptions for Traefik labels
- Better error handling and debug logging for troubleshooting
- IP address discovery for containers
- Proper configuration generation from VM/container labels

### Fixed

- Empty configuration issue when using `key=value` format
- Extended poll interval from 5s to 30s to reduce API load
- Package structure to match Traefik plugin standards

### Documentation

- Improved README with clear labeling instructions
- Added troubleshooting section
- Added examples for different routing scenarios

## [v0.4.5] - 2024-03-21

### Added

- Support for Proxmox VE 8.0 and newer versions
- Improved error handling and logging throughout the plugin
- Better configuration validation with detailed error messages
- Initial configuration update before starting the polling interval
- Panic recovery in provider goroutines for better stability
- Minimum poll interval check (5 seconds) to prevent API overload
- Detailed logging for VM and container scanning operations

### Changed

- Default poll interval increased from 5s to 30s to reduce API load
- Improved error messages with proper error wrapping
- Better organization of code structure following Traefik plugin best practices
- Enhanced configuration validation with more specific error messages
- Updated logging messages to be more descriptive and informative
- Improved error handling in goroutines with proper context cancellation

### Fixed

- Potential race conditions in configuration updates
- Memory leaks in long-running operations
- Error handling in network interface scanning
- Configuration validation for required fields
- Proper cleanup of resources in Stop() method

### Security

- Added validation for API endpoint and token configuration
- Improved SSL validation handling
- Better error handling for API authentication failures

### Documentation

- Updated README with improved configuration examples
- Added more detailed logging information
- Better documentation of configuration options

### Dependencies

- Updated to use latest Traefik plugin interfaces
- Improved compatibility with newer Go versions

### Notes

- This version requires Traefik v2.0 or newer
- The plugin now follows standard Traefik plugin naming conventions
- Improved stability and reliability for production environments 