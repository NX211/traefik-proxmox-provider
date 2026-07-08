# Changelog

## [Unreleased]

### Added

- Multiple Proxmox endpoints in a single plugin instance via the `nodes` list, for VMs/containers spread across standalone (non-clustered) Proxmox hosts. Based on the approach from PR #43 by @jonmilele.
- Auto-generated router/service IDs are prefixed with the endpoint `name` when multiple endpoints are configured, so identical VM names/IDs on different hosts don't collide.
- Per-endpoint connection log: the Proxmox VE version and logical endpoint name are logged on the first successful poll of each endpoint.

### Changed

- An unreachable Proxmox endpoint no longer aborts provider startup. It is logged as a warning and retried on every poll, and a failing endpoint never affects routes discovered on the remaining endpoints. Structurally invalid configuration (missing endpoint/token fields) is still a fatal error, per entry.
- When every configured endpoint is unreachable during a poll, the previous dynamic configuration is kept instead of pushing an empty one.

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