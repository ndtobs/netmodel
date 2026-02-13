// Package model defines the netmodel YAML data model types
package model

import "time"

// DeviceModel is the root model for a single device
type DeviceModel struct {
	Metadata      Metadata              `yaml:"metadata,omitempty"`
	Interfaces    map[string]*Interface `yaml:"interfaces,omitempty"`
	BGP           *BGP                  `yaml:"bgp,omitempty"`
	OSPF          *OSPF                 `yaml:"ospf,omitempty"`
	EVPN          *EVPN                 `yaml:"evpn,omitempty"`
	System        *System               `yaml:"system,omitempty"`
	RoutingPolicy *RoutingPolicy        `yaml:"routing_policy,omitempty"`
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

// ============================================================================
// Interfaces
// ============================================================================

// Interface represents an interface configuration
type Interface struct {
	Description string          `yaml:"description,omitempty"`
	Enabled     *bool           `yaml:"enabled,omitempty"`
	MTU         int             `yaml:"mtu,omitempty"`
	Type        string          `yaml:"type,omitempty"`
	Speed       string          `yaml:"speed,omitempty"`
	Duplex      string          `yaml:"duplex,omitempty"`
	IPv4        *InterfaceIPv4  `yaml:"ipv4,omitempty"`
	IPv6        *InterfaceIPv6  `yaml:"ipv6,omitempty"`
	Ethernet    *EthernetConfig `yaml:"ethernet,omitempty"`
	LAG         *LAGConfig      `yaml:"lag,omitempty"`
	VXLAN       *VXLANConfig    `yaml:"vxlan,omitempty"`
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

// EthernetConfig represents ethernet-specific configuration
type EthernetConfig struct {
	PortSpeed    string `yaml:"port_speed,omitempty"`
	AutoNegotiate *bool  `yaml:"auto_negotiate,omitempty"`
	DuplexMode   string `yaml:"duplex_mode,omitempty"`
	MacAddress   string `yaml:"mac_address,omitempty"`
}

// LAGConfig represents LAG/port-channel membership
type LAGConfig struct {
	AggregateID string `yaml:"aggregate_id,omitempty"`
	LACPMode    string `yaml:"lacp_mode,omitempty"` // ACTIVE, PASSIVE
}

// VXLANConfig represents VXLAN tunnel interface configuration
type VXLANConfig struct {
	SourceInterface string     `yaml:"source_interface,omitempty"`
	UDPPort         int        `yaml:"udp_port,omitempty"`
	VNIMappings     []VNIMapping `yaml:"vni_mappings,omitempty"`
}

// VNIMapping represents a VNI to VLAN mapping
type VNIMapping struct {
	VNI  int `yaml:"vni"`
	VLAN int `yaml:"vlan,omitempty"`
	VRF  string `yaml:"vrf,omitempty"`
}

// ============================================================================
// BGP
// ============================================================================

// BGP represents the BGP configuration
type BGP struct {
	Global     BGPGlobal                `yaml:"global,omitempty"`
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
	Description    string       `yaml:"description,omitempty"`
	PeerAS         uint32       `yaml:"peer_as,omitempty"`
	LocalAS        uint32       `yaml:"local_as,omitempty"`
	PeerType       string       `yaml:"peer_type,omitempty"` // INTERNAL, EXTERNAL
	UpdateSource   string       `yaml:"update_source,omitempty"`
	NextHopSelf    *bool        `yaml:"next_hop_self,omitempty"`
	SendCommunity  string       `yaml:"send_community,omitempty"` // STANDARD, EXTENDED, BOTH, NONE
	AFI            []BGPAfiSafi `yaml:"afi_safi,omitempty"`
	Timers         *BGPTimers   `yaml:"timers,omitempty"`
	EBGPMultihop   int          `yaml:"ebgp_multihop,omitempty"`
	RouteReflector *bool        `yaml:"route_reflector_client,omitempty"`
}

// BGPNeighbor represents a BGP neighbor
type BGPNeighbor struct {
	Description    string       `yaml:"description,omitempty"`
	Enabled        *bool        `yaml:"enabled,omitempty"`
	PeerAS         uint32       `yaml:"peer_as,omitempty"`
	PeerGroup      string       `yaml:"peer_group,omitempty"`
	LocalAS        uint32       `yaml:"local_as,omitempty"`
	PeerType       string       `yaml:"peer_type,omitempty"` // INTERNAL, EXTERNAL
	UpdateSource   string       `yaml:"update_source,omitempty"`
	NextHopSelf    *bool        `yaml:"next_hop_self,omitempty"`
	SendCommunity  string       `yaml:"send_community,omitempty"`
	AFI            []BGPAfiSafi `yaml:"afi_safi,omitempty"`
	Timers         *BGPTimers   `yaml:"timers,omitempty"`
	EBGPMultihop   int          `yaml:"ebgp_multihop,omitempty"`
	ImportPolicy   string       `yaml:"import_policy,omitempty"`
	ExportPolicy   string       `yaml:"export_policy,omitempty"`
	RouteReflector *bool        `yaml:"route_reflector_client,omitempty"`
}

// BGPAfiSafi represents address family configuration
type BGPAfiSafi struct {
	Name         string `yaml:"name"` // IPV4_UNICAST, IPV6_UNICAST, L2VPN_EVPN, etc.
	Enabled      *bool  `yaml:"enabled,omitempty"`
	ImportPolicy string `yaml:"import_policy,omitempty"`
	ExportPolicy string `yaml:"export_policy,omitempty"`
}

// BGPTimers represents BGP timer configuration
type BGPTimers struct {
	HoldTime      int `yaml:"hold_time,omitempty"`
	KeepaliveTime int `yaml:"keepalive_time,omitempty"`
	ConnectRetry  int `yaml:"connect_retry,omitempty"`
}

// ============================================================================
// System
// ============================================================================

// System represents system-level configuration
type System struct {
	Hostname   string   `yaml:"hostname,omitempty"`
	DomainName string   `yaml:"domain_name,omitempty"`
	NTP        *NTP     `yaml:"ntp,omitempty"`
	DNS        *DNS     `yaml:"dns,omitempty"`
	AAA        *AAA     `yaml:"aaa,omitempty"`
	Logging    *Logging `yaml:"logging,omitempty"`
}

// NTP represents NTP configuration
type NTP struct {
	Enabled bool        `yaml:"enabled,omitempty"`
	Servers []NTPServer `yaml:"servers,omitempty"`
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

// AAA represents authentication/authorization config
type AAA struct {
	Users []User `yaml:"users,omitempty"`
}

// User represents a local user account
type User struct {
	Username string `yaml:"username"`
	Role     string `yaml:"role,omitempty"`
	SSHKey   string `yaml:"ssh_key,omitempty"`
}

// Logging represents syslog/logging configuration
type Logging struct {
	Servers []LogServer `yaml:"servers,omitempty"`
}

// LogServer represents a syslog server
type LogServer struct {
	Address  string `yaml:"address"`
	Port     int    `yaml:"port,omitempty"`
	Protocol string `yaml:"protocol,omitempty"` // UDP, TCP
	Facility string `yaml:"facility,omitempty"`
}

// ============================================================================
// OSPF
// ============================================================================

// OSPF represents OSPF configuration
type OSPF struct {
	Global *OSPFGlobal        `yaml:"global,omitempty"`
	Areas  map[string]*OSPFArea `yaml:"areas,omitempty"`
}

// OSPFGlobal represents global OSPF configuration
type OSPFGlobal struct {
	RouterID string `yaml:"router_id,omitempty"`
}

// OSPFArea represents an OSPF area
type OSPFArea struct {
	Identifier string                   `yaml:"identifier,omitempty"`
	Type       string                   `yaml:"type,omitempty"` // NORMAL, STUB, NSSA
	Interfaces map[string]*OSPFInterface `yaml:"interfaces,omitempty"`
}

// OSPFInterface represents an interface in OSPF
type OSPFInterface struct {
	NetworkType string `yaml:"network_type,omitempty"` // BROADCAST, POINT_TO_POINT, etc.
	Passive     *bool  `yaml:"passive,omitempty"`
	Cost        int    `yaml:"cost,omitempty"`
	Priority    int    `yaml:"priority,omitempty"`
	HelloInterval int  `yaml:"hello_interval,omitempty"`
	DeadInterval  int  `yaml:"dead_interval,omitempty"`
}

// ============================================================================
// EVPN / VXLAN
// ============================================================================

// EVPN represents EVPN/VXLAN configuration
type EVPN struct {
	VTEPSource string               `yaml:"vtep_source,omitempty"` // Source interface for VTEP
	UDPPort    int                  `yaml:"udp_port,omitempty"`    // VXLAN UDP port (default 4789)
	VLANVNIs   map[string]*VLANVNI  `yaml:"vlan_vnis,omitempty"`   // VLAN to VNI mappings
	VRFVNIs    map[string]*VRFVNI   `yaml:"vrf_vnis,omitempty"`    // VRF L3VNI mappings
}

// VLANVNI represents a VLAN to VNI mapping (L2VNI)
type VLANVNI struct {
	VLAN            int      `yaml:"vlan"`
	VNI             int      `yaml:"vni"`
	RD              string   `yaml:"rd,omitempty"`
	RouteTargetBoth []string `yaml:"route_target_both,omitempty"`
	RouteTargetImport []string `yaml:"route_target_import,omitempty"`
	RouteTargetExport []string `yaml:"route_target_export,omitempty"`
}

// VRFVNI represents a VRF to L3VNI mapping
type VRFVNI struct {
	VRF               string   `yaml:"vrf"`
	VNI               int      `yaml:"vni"`
	RD                string   `yaml:"rd,omitempty"`
	RouteTargetImport []string `yaml:"route_target_import,omitempty"`
	RouteTargetExport []string `yaml:"route_target_export,omitempty"`
}

// ============================================================================
// Routing Policy
// ============================================================================

// RoutingPolicy represents routing policy configuration
type RoutingPolicy struct {
	DefinedSets       *DefinedSets       `yaml:"defined_sets,omitempty"`
	PolicyDefinitions []PolicyDefinition `yaml:"policy_definitions,omitempty"`
}

// DefinedSets contains prefix-sets, community-sets, as-path-sets
type DefinedSets struct {
	PrefixSets    []PrefixSet    `yaml:"prefix_sets,omitempty"`
	CommunitySets []CommunitySet `yaml:"community_sets,omitempty"`
	ASPathSets    []ASPathSet    `yaml:"as_path_sets,omitempty"`
}

// PrefixSet represents a prefix-list
type PrefixSet struct {
	Name     string   `yaml:"name"`
	Mode     string   `yaml:"mode,omitempty"` // IPV4, IPV6
	Prefixes []Prefix `yaml:"prefixes,omitempty"`
}

// Prefix represents a single prefix entry
type Prefix struct {
	Prefix          string `yaml:"prefix"`
	MaskLengthRange string `yaml:"mask_range,omitempty"` // e.g., "24..32"
}

// CommunitySet represents a community-list
type CommunitySet struct {
	Name    string   `yaml:"name"`
	Members []string `yaml:"members,omitempty"`
}

// ASPathSet represents an as-path-list
type ASPathSet struct {
	Name    string   `yaml:"name"`
	Members []string `yaml:"members,omitempty"`
}

// PolicyDefinition represents a route-map
type PolicyDefinition struct {
	Name       string            `yaml:"name"`
	Statements []PolicyStatement `yaml:"statements,omitempty"`
}

// PolicyStatement represents a route-map entry/sequence
type PolicyStatement struct {
	Name       string           `yaml:"name"` // sequence number or name
	Conditions *PolicyConditions `yaml:"conditions,omitempty"`
	Actions    *PolicyActions    `yaml:"actions,omitempty"`
}

// PolicyConditions represents match conditions
type PolicyConditions struct {
	MatchPrefixSet    string `yaml:"match_prefix_set,omitempty"`
	MatchCommunitySet string `yaml:"match_community_set,omitempty"`
	MatchASPathSet    string `yaml:"match_as_path_set,omitempty"`
	MatchNextHop      string `yaml:"match_next_hop,omitempty"`
}

// PolicyActions represents route-map actions
type PolicyActions struct {
	Accept        *bool  `yaml:"accept,omitempty"`
	Reject        *bool  `yaml:"reject,omitempty"`
	SetLocalPref  int    `yaml:"set_local_pref,omitempty"`
	SetMED        int    `yaml:"set_med,omitempty"`
	SetNextHop    string `yaml:"set_next_hop,omitempty"`
	SetCommunity  string `yaml:"set_community,omitempty"`
	SetASPathPrepend string `yaml:"set_as_path_prepend,omitempty"`
}
