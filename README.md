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

## Installation

1. Add the plugin to your Traefik configuration:

```yaml
experimental:
  plugins:
    traefik-proxmox-provider:
      moduleName: github.com/NX211/traefik-proxmox-provider
      version: v0.5.5
```

2. Configure the provider in your dynamic configuration:

```yaml
# Dynamic configuration
providers:
  plugin:
    traefik-proxmox-provider:
      pollInterval: "30s"
      apiEndpoint: "https://proxmox.example.com"
      apiTokenId: "root@pam!traefik"
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
| `apiTokenId` | `string` | - | The API token ID (e.g., "root@pam!traefik") |
| `apiToken` | `string` | - | The API token secret |
| `apiLogging` | `string` | `"info"` | Log level for API operations ("debug" or "info") |
| `apiValidateSSL` | `string` | `"true"` | Whether to validate SSL certificates |

## Usage

1. Create an API token in Proxmox VE:
   - Go to Datacenter -> Permissions -> API Tokens
   - Add a new token with appropriate permissions
   - Copy the token ID and secret

2. Configure the provider in your Traefik configuration:
   - Set the `apiEndpoint` to your Proxmox VE server URL
   - Set the `apiTokenId` and `apiToken` from step 1
   - Adjust other options as needed

3. **Very Important**: Add Traefik labels to your VMs/containers:
   - Edit your VM/container in Proxmox VE
   - Go to the "Options" tab and edit "Description"
   - Add one Traefik label per line with the format `traefik.key=value`
   - **At minimum** add `traefik.enable=true` to enable Traefik for this VM/container

4. Restart Traefik to load the new configuration

## VM/Container Labeling

The provider looks for Traefik labels in the VM/container description field. Each line in the description starting with `traefik.` will be treated as a Traefik label.

### Required Labels

- `traefik.enable=true` - Without this label, the VM/container will be ignored

### Common Labels

- `traefik.http.routers.rule=Host(`myapp.example.com`)` - The router rule for this service
- `traefik.http.services.port=8080` - The port to route traffic to (defaults to 80)

### Full Example of VM/Container Description

```
My application server
Some notes about this server

traefik.enable=true
traefik.http.routers.rule=Host(`myapp.example.com`)
traefik.http.services.port=8080
```

## How It Works

1. The provider connects to your Proxmox VE cluster via API
2. It discovers all running VMs and containers on all nodes
3. For each VM/container, it reads the description looking for Traefik labels
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
      apiTokenId: "root@pam!traefik"
      apiToken: "your-api-token"
      apiLogging: "debug"  # Use debug for troubleshooting
      apiValidateSSL: "true"
```

### VM/Container Label Examples

Simple web server:
```
traefik.enable=true
traefik.http.routers.rule=Host(`myapp.example.com`)
```

Custom port:
```
traefik.enable=true
traefik.http.routers.rule=Host(`api.example.com`)
traefik.http.services.port=3000
```

Multiple hosts:
```
traefik.enable=true
traefik.http.routers.rule=Host(`app.example.com`) || Host(`www.example.com`)
```

Path-based routing:
```
traefik.enable=true
traefik.http.routers.rule=Host(`example.com`) && PathPrefix(`/api`)
```

## Troubleshooting

If your services aren't being discovered:

1. Enable debug logging by setting `apiLogging: "debug"`
2. Check that VMs/containers have `traefik.enable=true` in their description
3. Verify that VMs/containers are in the "running" state
4. Check that the provider can successfully connect to your Proxmox API
5. Verify the API token has sufficient permissions

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
