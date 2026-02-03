package internal

import (
	"strings"
)

type ParsedConfig struct {
	Description string `json:"description,omitempty"`
}

type ParsedAgentInterfaces struct {
	Result []struct {
		IPAddresses []IP `json:"ip-addresses"`
	} `json:"result"`
}

type NodeStatus struct {
	Node string `json:"node"`
}

type VirtualMachine struct {
	VMID   uint64 `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Container struct {
	VMID   uint64 `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Version struct {
	Release string `json:"release"`
}

type Service struct {
	ID     uint64
	Name   string
	IPs    []IP
	Config map[string]string
}

type IP struct {
	Address     string `json:"ip-address,omitempty"`
	AddressType string `json:"ip-address-type,omitempty"`
	Prefix      uint64 `json:"prefix,omitempty"`
}

func NewService(id uint64, name string, config map[string]string) Service {
	return Service{ID: id, Name: name, Config: config, IPs: make([]IP, 0)}
}

func (pc *ParsedConfig) GetTraefikMap() map[string]string {
	const separator = "="

	// Normalize space-separated traefik labels (e.g. from OCI containers)
	// into newline-separated labels so they are parsed individually.
	normalized := strings.ReplaceAll(pc.Description, " traefik.", "\ntraefik.")

	m := make(map[string]string)
	lines := strings.Split(normalized, "\n")
	for _, line := range lines {
		key, value, found := strings.Cut(line, separator)
		if !found {
			continue
		}

		key = strings.Trim(key, "\" ")
		value = strings.Trim(value, "\" ")

		if strings.HasPrefix(strings.ToLower(key), "traefik.") {
			m[strings.ToLower(key)] = value
		}
	}
	return m
}

func (pai *ParsedAgentInterfaces) GetIPs() []IP {
	ips := make([]IP, 0)
	for _, r := range pai.Result {
		ips = append(ips, r.IPAddresses...)
	}
	return ips
}
