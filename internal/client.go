package internal

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv" // Added import
	"time"
)

// Log levels
const (
	LogLevelInfo  = "info"
	LogLevelDebug = "debug"
)

// ProxmoxClient represents a client to the Proxmox API
type ProxmoxClient struct {
	BaseURL     string
	TokenID     string
	Token       string
	HTTPClient  *http.Client
	LogLevel    string
	ValidateSSL bool
}

// NewProxmoxClient creates a new Proxmox API client
func NewProxmoxClient(apiEndpoint, tokenID, token string, validateSSL bool, logLevel string) *ProxmoxClient {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !validateSSL,
			},
		},
		Timeout: 30 * time.Second,
	}

	baseURL := fmt.Sprintf("%s/api2/json", apiEndpoint)
	if logLevel == LogLevelDebug {
		log.Printf("Creating new Proxmox client with base URL: %s", baseURL)
	}

	return &ProxmoxClient{
		BaseURL:     baseURL,
		TokenID:     tokenID,
		Token:       token,
		HTTPClient:  httpClient,
		LogLevel:    logLevel,
		ValidateSSL: validateSSL,
	}
}

// Do performs an HTTP request to the Proxmox API
func (c *ProxmoxClient) Do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	fullURL := c.BaseURL + path

	if c.LogLevel == LogLevelDebug {
		log.Printf("API Request: %s %s", method, fullURL)
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.TokenID, c.Token))
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if c.LogLevel == LogLevelDebug {
			log.Printf("API Response: %s", string(respBody))
		}

		err = json.Unmarshal(respBody, result)
		if err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// Get performs a GET request to the Proxmox API
func (c *ProxmoxClient) Get(ctx context.Context, path string, result interface{}) error {
	return c.Do(ctx, http.MethodGet, path, nil, result)
}

// GetVersion retrieves the Proxmox version
func (c *ProxmoxClient) GetVersion(ctx context.Context) (*Version, error) {
	var response struct {
		Data Version `json:"data"`
	}
	err := c.Get(ctx, "/version", &response)
	if err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// GetNodes retrieves all nodes in the Proxmox cluster
func (c *ProxmoxClient) GetNodes(ctx context.Context) ([]NodeStatus, error) {
	var response struct {
		Data []NodeStatus `json:"data"`
	}
	err := c.Get(ctx, "/nodes", &response)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetVirtualMachines retrieves all VMs on a node
func (c *ProxmoxClient) GetVirtualMachines(ctx context.Context, nodeName string) ([]VirtualMachine, error) {
	var response struct {
		Data []VirtualMachine `json:"data"`
	}
	err := c.Get(ctx, fmt.Sprintf("/nodes/%s/qemu", nodeName), &response)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetContainers retrieves all containers on a node
func (c *ProxmoxClient) GetContainers(ctx context.Context, nodeName string) ([]Container, error) {
	var response struct {
		Data []Container `json:"data"`
	}
	err := c.Get(ctx, fmt.Sprintf("/nodes/%s/lxc", nodeName), &response)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetVMConfig retrieves the configuration of a VM
func (c *ProxmoxClient) GetVMConfig(ctx context.Context, nodeName string, vmID uint64) (*ParsedConfig, error) {
	var response struct {
		Data ParsedConfig `json:"data"`
	}
	err := c.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", nodeName, vmID), &response)
	if err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// GetContainerConfig retrieves the configuration of a container
func (c *ProxmoxClient) GetContainerConfig(ctx context.Context, nodeName string, vmID uint64) (*ParsedConfig, error) {
	var response struct {
		Data ParsedConfig `json:"data"`
	}
	err := c.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/config", nodeName, vmID), &response)
	if err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// GetVMNetworkInterfaces retrieves network interfaces from a VM using the QEMU guest agent
func (c *ProxmoxClient) GetVMNetworkInterfaces(ctx context.Context, nodeName string, vmID uint64) (*ParsedAgentInterfaces, error) {
	var response struct {
		Data ParsedAgentInterfaces `json:"data"`
	}
	err := c.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", nodeName, vmID), &response)
	if err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// GetContainerNetworkInterfaces retrieves network interfaces from a container
func (c *ProxmoxClient) GetContainerNetworkInterfaces(ctx context.Context, nodeName string, vmID uint64) (*ParsedAgentInterfaces, error) {
	var response struct {
		Data []struct {
			Name            string `json:"name"`
			HardwareAddress string `json:"hardware-address"`
			Inet            string `json:"inet"`
			Inet6           string `json:"inet6"`
			IPAddresses     []struct {
				Address     string      `json:"ip-address,omitempty"`
				AddressType string      `json:"ip-address-type,omitempty"`
				Prefix      json.Number `json:"prefix,omitempty"` // Use json.Number
			} `json:"ip-addresses"`
			HWAddr string `json:"hwaddr"`
		} `json:"data"`
	}
	err := c.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", nodeName, vmID), &response)
	if err != nil {
		return nil, err
	}

	result := &ParsedAgentInterfaces{
		Result: make([]struct {
			IPAddresses []IP `json:"ip-addresses"`
		}, 0),
	}

	for _, iface := range response.Data {
		var ips []IP
		for _, ip := range iface.IPAddresses {
			prefixUint, err := strconv.ParseUint(ip.Prefix.String(), 10, 64) // Changed to use strconv.ParseUint
			if err != nil {
				// Log error but continue, as some IPs might be valid
				if c.LogLevel == LogLevelDebug {
					log.Printf("DEBUG: Failed to parse prefix string '%s' to uint64 for IP %s: %v", ip.Prefix.String(), ip.Address, err)
				}
				continue
			}
			ips = append(ips, IP{
				Address:     ip.Address,
				AddressType: ip.AddressType,
				Prefix:      prefixUint,
			})
		}

		result.Result = append(result.Result, struct {
			IPAddresses []IP `json:"ip-addresses"`
		}{
			IPAddresses: ips,
		})
	}

	return result, nil
}
