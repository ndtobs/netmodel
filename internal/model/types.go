// Package model defines the netmodel YAML data model types
package model

import "time"

// DeviceModel is the root model for a single device
type DeviceModel struct {
	Metadata   Metadata              `yaml:"metadata,omitempty"`
	Interfaces map[string]*Interface `yaml:"interfaces,omitempty"`
	BGP        *BGP                  `yaml:"bgp,omitempty"`
	System     *System               `yaml:"system,omitempty"`
}

// Metadata contains device identification
type Metadata struct {
	Hostname        string    `yaml:"hostname,omitempty"`
	Model           string    `yaml:"model,omitempty"`
	Version         string    `yaml:"version,omitempty"`
	Serial          string    `yaml:"serial,omitempty"`
	Platform        string    `yaml:"platform,omitempty"`
	ExportedAt      time.Time `yaml:"exported_at,omitempty"`
	NetmodelVersion string    `yaml:"netmodel_version,omitempty"`
}

// Interface represents an interface configuration
type Interface struct {
	Description string        `yaml:"description,omitempty"`
	Enabled     *bool         `yaml:"enabled,omitempty"`
	MTU         int           `yaml:"mtu,omitempty"`
	Type        string        `yaml:"type,omitempty"`
	IPv4        *InterfaceIPv4 `yaml:"ipv4,omitempty"`
	IPv6        *InterfaceIPv6 `yaml:"ipv6,omitempty"`
}

// InterfaceIPv4 represents IPv4 configuration on an interface
type InterfaceIPv4 struct {
	Addresses []IPv4Address `yaml:"addresses,omitempty"`
}

// IPv4Address represents an IPv4 address
type IPv4Address struct {
	IP           string `yaml:"ip"`
	PrefixLength int    `yaml:"prefix_length"`
}

// InterfaceIPv6 represents IPv6 configuration on an interface
type InterfaceIPv6 struct {
	Addresses []IPv6Address `yaml:"addresses,omitempty"`
}

// IPv6Address represents an IPv6 address
type IPv6Address struct {
	IP           string `yaml:"ip"`
	PrefixLength int    `yaml:"prefix_length"`
}

// BGP represents the BGP configuration
type BGP struct {
	Global     BGPGlobal              `yaml:"global,omitempty"`
	PeerGroups map[string]*BGPPeerGroup `yaml:"peer_groups,omitempty"`
	Neighbors  map[string]*BGPNeighbor  `yaml:"neighbors,omitempty"`
}

// BGPGlobal represents global BGP configuration
type BGPGlobal struct {
	AS       uint32 `yaml:"as,omitempty"`
	RouterID string `yaml:"router_id,omitempty"`
}

// BGPPeerGroup represents a BGP peer group
type BGPPeerGroup struct {
	Description string `yaml:"description,omitempty"`
	PeerAS      uint32 `yaml:"peer_as,omitempty"`
}

// BGPNeighbor represents a BGP neighbor
type BGPNeighbor struct {
	Description string `yaml:"description,omitempty"`
	Enabled     *bool  `yaml:"enabled,omitempty"`
	PeerAS      uint32 `yaml:"peer_as,omitempty"`
	PeerGroup   string `yaml:"peer_group,omitempty"`
	LocalAS     uint32 `yaml:"local_as,omitempty"`
}

// System represents system-level configuration
type System struct {
	Hostname   string   `yaml:"hostname,omitempty"`
	DomainName string   `yaml:"domain_name,omitempty"`
	NTP        *NTP     `yaml:"ntp,omitempty"`
	DNS        *DNS     `yaml:"dns,omitempty"`
}

// NTP represents NTP configuration
type NTP struct {
	Enabled bool         `yaml:"enabled,omitempty"`
	Servers []NTPServer  `yaml:"servers,omitempty"`
}

// NTPServer represents an NTP server
type NTPServer struct {
	Address string `yaml:"address"`
	Prefer  bool   `yaml:"prefer,omitempty"`
}

// DNS represents DNS configuration
type DNS struct {
	Servers []string `yaml:"servers,omitempty"`
	Search  []string `yaml:"search,omitempty"`
}
