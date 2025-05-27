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

type ContainerNetworkInterface struct {
	Name            string `json:"name"`
	HardwareAddress string `json:"hardware-address"`
	Inet            string `json:"inet"`
	Inet6           string `json:"inet6"`
	IPAddresses     []IP   `json:"ip-addresses"`
}

type Node struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"node,omitempty"`
	Status string `json:"status,omitempty"`
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
	Name   string
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

	m := make(map[string]string)
	lines := strings.Split(pc.Description, "\n")
	for _, line := range lines {
		key, value, found := strings.Cut(line, separator)
		if !found {
			continue
		}

		key = strings.Trim(key, "\" ")
		value = strings.Trim(value, "\" ")

		if strings.HasPrefix(key, "traefik.") {
			m[key] = value
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
