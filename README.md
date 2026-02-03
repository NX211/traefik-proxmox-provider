# Traefik Proxmox Provider

![Traefik Proxmox Provider](https://raw.githubusercontent.com/nx211/traefik-proxmox-provider/main/.assets/logo.png)

A Traefik provider that automatically configures routing based on Proxmox VE virtual machines and containers.

## Features

- Automatically discovers Proxmox VE virtual machines and containers
- Configures routing based on VM/container metadata
- Supports both HTTP and HTTPS endpoints
- Configurable polling interval
- SSL validation options
- Logging configuration
- Full support for Traefik's routing, middleware, and TLS options

## Installation

1. Add the plugin to your Traefik configuration:

```yaml
experimental:
  plugins:
    traefik-proxmox-provider:
      moduleName: github.com/NX211/traefik-proxmox-provider
      version: v0.7.0
```

2. Configure the provider in your dynamic configuration:

```yaml
# Dynamic configuration
providers:
  plugin:
    traefik-proxmox-provider:
      pollInterval: "30s"
      apiEndpoint: "https://proxmox.example.com"
      apiTokenId: "root@pam!traefik_prod"
      apiToken: "your-api-token"
      apiLogging: "info"
      apiValidateSSL: "true"
```

## Configuration

### Provider Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `pollInterval` | `string` | `"30s"` | How often to poll the Proxmox API for changes |
| `apiEndpoint` | `string` | - | The URL of your Proxmox VE API |
| `apiTokenId` | `string` | - | The API token ID (e.g., "root@pam!traefik_prod") |
| `apiToken` | `string` | - | The API token secret |
| `apiLogging` | `string` | `"info"` | Log level for API operations ("debug" or "info") |
| `apiValidateSSL` | `string` | `"true"` | Whether to validate SSL certificates |

## Proxmox API Token Setup

The Traefik Proxmox Provider needs an API token with specific permissions to read VM and container information. Here's how to set up the proper token and permissions:

```bash
# Create a role for Traefik provider with minimum required permissions

## For Proxmox VE 8.x or earlier:
pveum role add traefik-provider -privs "VM.Audit,VM.Monitor,Sys.Audit,Datastore.Audit"

## For Proxmox VE 9.0 or later (VM.Monitor was removed and replaced with granular privileges):
pveum role add traefik-provider -privs "VM.Audit,VM.GuestAgent.Audit,Sys.Audit,Datastore.Audit"

# Create an API token for your user (replace with your actual username)
pveum user token add root@pam traefik_prod

# Assign the role to the token for all paths
pveum acl modify / -token 'root@pam!traefik_prod' -role traefik-provider
```

> **Note:** If you are upgrading from Proxmox VE 8.x to 9.x, you must update your API token role. The `VM.Monitor` privilege was removed in PVE 9.0 and replaced with `VM.GuestAgent.Audit` (required for reading VM network interfaces via the QEMU guest agent). Without this, the provider will receive 403 errors when discovering VM IP addresses.

Make sure to save the API token value when it's displayed, as it won't be shown again.

## Usage

1. Create an API token in Proxmox VE as described above

2. Configure the provider in your Traefik configuration:
   - Set the `apiEndpoint` to your Proxmox VE server URL
   - Set the `apiTokenId` and `apiToken` from step 1
   - Adjust other options as needed

3. **Very Important**: Add Traefik labels to your VMs/containers:
   - Edit your VM/container in Proxmox VE
   - Go to the "Summary" page and edit the "Notes" box by clicking the pencil icon
   - Add one Traefik label per line with the format `traefik.key=value`
   - **At minimum** add `traefik.enable=true` to enable Traefik for this VM/container

4. Restart Traefik to load the new configuration

## VM/Container Labeling

The provider looks for Traefik labels in the VM/container notes field. Each line in the Notes field starting with `traefik.` will be treated as a Traefik label.

### Required Labels

- `traefik.enable=true` - Without this label, the VM/container will be ignored

### Common Labels

- `traefik.http.routers.<name>.rule=Host(`myapp.example.com`)` - The router rule for this service
- `traefik.http.services.<name>.loadbalancer.server.port=8080` - The port to route traffic to (defaults to 80)

### Advanced Label Examples

#### Named Routers and Services

```
traefik.enable=true
traefik.http.routers.myapp.rule=Host(`myapp.example.com`)
traefik.http.services.appservice.loadbalancer.server.port=8080
traefik.http.routers.myapp.service=appservice
```

#### EntryPoints

```
traefik.http.routers.myapp.entrypoints=websecure
```

#### Middlewares

```
traefik.http.routers.myapp.middlewares=compression,auth@file
```

#### TLS Configuration

```
traefik.http.routers.myapp.tls=true
traefik.http.routers.myapp.tls.certresolver=myresolver
traefik.http.routers.myapp.tls.domains=example.com
traefik.http.routers.myapp.tls.options=tlsoptions@file
```

#### Health Checks

```
traefik.http.services.myservice.loadbalancer.healthcheck.path=/health
traefik.http.services.myservice.loadbalancer.healthcheck.interval=10s
traefik.http.services.myservice.loadbalancer.healthcheck.timeout=5s
```

#### Sticky Sessions

```
traefik.http.services.myservice.loadbalancer.sticky.cookie.name=session
traefik.http.services.myservice.loadbalancer.sticky.cookie.secure=true
traefik.http.services.myservice.loadbalancer.sticky.cookie.httponly=true
```

#### HTTPS Backend Services

```
traefik.http.services.myservice.loadbalancer.server.scheme=https
```

### Full Example of VM/Container Notes

```
My application server
Some notes about this server

traefik.enable=true
traefik.http.routers.myapp.rule=Host(`myapp.example.com`)
traefik.http.routers.myapp.entrypoints=websecure
traefik.http.routers.myapp.middlewares=auth@file,compression
traefik.http.routers.myapp.tls=true
traefik.http.routers.myapp.tls.certresolver=myresolver
traefik.http.services.myapp.loadbalancer.server.port=8080
traefik.http.services.myapp.loadbalancer.healthcheck.path=/health
```

## How It Works

1. The provider connects to your Proxmox VE cluster via API
2. It discovers all running VMs and containers on all nodes
3. For each VM/container, it reads the notes field looking for Traefik labels
4. If `traefik.enable=true` is found, it creates a Traefik router and service
5. The provider attempts to get IP addresses for the VM/container 
6. If IPs are found, they're used as server URLs; otherwise, the VM/container hostname is used
7. This process repeats according to the configured poll interval

## Examples

### Basic Configuration

```yaml
providers:
  plugin:
    traefik-proxmox-provider:
      pollInterval: "30s"
      apiEndpoint: "https://proxmox.example.com"
      apiTokenId: "root@pam!traefik_prod"
      apiToken: "your-api-token"
      apiLogging: "debug"  # Use debug for troubleshooting
      apiValidateSSL: "true"
```

### VM/Container Label Examples

Simple web server:
```
traefik.enable=true
traefik.http.routers.app.rule=Host(`myapp.example.com`)
```

Secure website with HTTPS:
```
traefik.enable=true
traefik.http.routers.secure.rule=Host(`secure.example.com`)
traefik.http.routers.secure.entrypoints=websecure
traefik.http.routers.secure.tls=true
traefik.http.routers.secure.tls.certresolver=dnschallenge
```

API with authentication and rate limiting:
```
traefik.enable=true
traefik.http.routers.api.rule=Host(`api.example.com`)
traefik.http.routers.api.middlewares=auth@file,ratelimit@file
traefik.http.services.api.loadbalancer.server.port=3000
```

Multiple hosts with path-based routing:
```
traefik.enable=true
traefik.http.routers.multi.rule=Host(`example.com`,`www.example.com`) && PathPrefix(`/api`)
traefik.http.routers.multi.priority=100
```

## Troubleshooting

If your services aren't being discovered:

1. **Enable debug logging**: Set `apiLogging: "debug"` and check Traefik's log file
2. **Verify VM/container config**: Must have `traefik.enable=true` in notes and be in "running" state
3. **Test API access** from the Traefik host:
   ```bash
   curl -k -H "Authorization: PVEAPIToken=root@pam!traefik_prod=YOUR-TOKEN" \
     https://proxmox:8006/api2/json/nodes
   ```
4. **Check token permissions**: Verify in Proxmox UI under **Datacenter → Permissions → API Tokens**
5. **Provider config location**: The plugin config belongs in Traefik's **static** config (`traefik.yaml`), not dynamic config

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
