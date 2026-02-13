// Package dedup extracts common configuration across devices for Ansible group_vars
package dedup

import (
	"reflect"

	"github.com/ndtobs/netmodel/internal/model"
)

// Result contains the deduplicated output
type Result struct {
	// Common contains config identical across ALL devices
	Common *model.DeviceModel

	// GroupCommon contains config identical within each group (excluding Common)
	// Key is group name
	GroupCommon map[string]*model.DeviceModel

	// HostSpecific contains per-host config after dedup
	// Key is hostname
	HostSpecific map[string]*model.DeviceModel
}

// Deduplicate analyzes models and extracts common configuration
// groups maps group names to lists of hostnames in that group
func Deduplicate(models map[string]*model.DeviceModel, groups map[string][]string) *Result {
	result := &Result{
		Common:       &model.DeviceModel{},
		GroupCommon:  make(map[string]*model.DeviceModel),
		HostSpecific: make(map[string]*model.DeviceModel),
	}

	if len(models) == 0 {
		return result
	}

	// Get list of all hostnames
	var allHosts []string
	for h := range models {
		allHosts = append(allHosts, h)
	}

	// Step 1: Find config common to ALL devices
	result.Common = findCommon(models, allHosts)

	// Step 2: For each group, find config common within that group (minus global common)
	for groupName, groupHosts := range groups {
		if len(groupHosts) < 2 {
			continue // No point deduping a single-host group
		}
		groupCommon := findCommon(models, groupHosts)
		// Subtract what's already in global common
		groupCommon = subtract(groupCommon, result.Common)
		if !isEmpty(groupCommon) {
			result.GroupCommon[groupName] = groupCommon
		}
	}

	// Step 3: For each host, keep only what's not in common or group common
	for host, dm := range models {
		specific := deepCopy(dm)

		// Subtract global common
		specific = subtract(specific, result.Common)

		// Subtract group common for any groups this host belongs to
		for groupName, groupHosts := range groups {
			if contains(groupHosts, host) {
				if gc, ok := result.GroupCommon[groupName]; ok {
					specific = subtract(specific, gc)
				}
			}
		}

		result.HostSpecific[host] = specific
	}

	return result
}

// findCommon returns a model containing only config identical across all specified hosts
func findCommon(models map[string]*model.DeviceModel, hosts []string) *model.DeviceModel {
	if len(hosts) == 0 {
		return &model.DeviceModel{}
	}

	// Start with a copy of the first host's model
	first := models[hosts[0]]
	if first == nil {
		return &model.DeviceModel{}
	}

	common := &model.DeviceModel{}

	// Compare System config
	if first.System != nil {
		common.System = findCommonSystem(models, hosts)
	}

	// Compare BGP config
	if first.BGP != nil {
		common.BGP = findCommonBGP(models, hosts)
	}

	// Compare RoutingPolicy config
	if first.RoutingPolicy != nil {
		common.RoutingPolicy = findCommonRoutingPolicy(models, hosts)
	}

	// Note: Interfaces, OSPF, EVPN are typically device-specific (different interface names, areas, etc.)
	// We could add dedup for those if patterns emerge, but for now they stay in host_vars

	return common
}

// findCommonSystem extracts system config common across hosts
func findCommonSystem(models map[string]*model.DeviceModel, hosts []string) *model.System {
	var systems []*model.System
	for _, h := range hosts {
		if m := models[h]; m != nil && m.System != nil {
			systems = append(systems, m.System)
		}
	}
	if len(systems) != len(hosts) {
		return nil // Not all hosts have system config
	}

	common := &model.System{}

	// Check NTP - compare entire NTP config as a unit
	if systems[0].NTP != nil && allEqual(systems, func(s *model.System) interface{} { return s.NTP }) {
		common.NTP = deepCopyNTP(systems[0].NTP)
	}

	// Check DNS
	if systems[0].DNS != nil && allEqual(systems, func(s *model.System) interface{} { return s.DNS }) {
		common.DNS = deepCopyDNS(systems[0].DNS)
	}

	// Check AAA
	if systems[0].AAA != nil && allEqual(systems, func(s *model.System) interface{} { return s.AAA }) {
		common.AAA = deepCopyAAA(systems[0].AAA)
	}

	// Check Logging
	if systems[0].Logging != nil && allEqual(systems, func(s *model.System) interface{} { return s.Logging }) {
		common.Logging = deepCopyLogging(systems[0].Logging)
	}

	// Hostname and DomainName are typically per-device, skip

	if isEmpty(common) {
		return nil
	}
	return common
}

// findCommonBGP extracts BGP config common across hosts
func findCommonBGP(models map[string]*model.DeviceModel, hosts []string) *model.BGP {
	var bgps []*model.BGP
	for _, h := range hosts {
		if m := models[h]; m != nil && m.BGP != nil {
			bgps = append(bgps, m.BGP)
		}
	}
	if len(bgps) != len(hosts) {
		return nil
	}

	common := &model.BGP{}

	// BGP Global (AS, router_id) - typically router_id differs, AS might be same
	// Only extract if completely identical
	if allEqual(bgps, func(b *model.BGP) interface{} { return b.Global }) {
		common.Global = bgps[0].Global
	}

	// Peer Groups - compare each peer group by name
	if bgps[0].PeerGroups != nil {
		commonPGs := make(map[string]*model.BGPPeerGroup)
		for pgName, pg := range bgps[0].PeerGroups {
			// Check if all hosts have this peer group with identical config
			allHave := true
			for _, b := range bgps[1:] {
				if b.PeerGroups == nil {
					allHave = false
					break
				}
				otherPG, ok := b.PeerGroups[pgName]
				if !ok || !reflect.DeepEqual(pg, otherPG) {
					allHave = false
					break
				}
			}
			if allHave {
				commonPGs[pgName] = deepCopyPeerGroup(pg)
			}
		}
		if len(commonPGs) > 0 {
			common.PeerGroups = commonPGs
		}
	}

	// Neighbors are typically device-specific (different peer IPs), skip

	if isEmpty(common) {
		return nil
	}
	return common
}

// findCommonRoutingPolicy extracts routing policy config common across hosts
func findCommonRoutingPolicy(models map[string]*model.DeviceModel, hosts []string) *model.RoutingPolicy {
	var policies []*model.RoutingPolicy
	for _, h := range hosts {
		if m := models[h]; m != nil && m.RoutingPolicy != nil {
			policies = append(policies, m.RoutingPolicy)
		}
	}
	if len(policies) != len(hosts) {
		return nil
	}

	// Compare entire routing policy as a unit (prefix-sets, community-sets, policies)
	// These are often identical across devices in the same role
	if allEqualRP(policies) {
		return deepCopyRoutingPolicy(policies[0])
	}

	// TODO: Could do finer-grained comparison per prefix-set, per policy, etc.
	return nil
}

// Helper: check if all items are equal using a selector function
func allEqual[T any](items []T, selector func(T) interface{}) bool {
	if len(items) < 2 {
		return true
	}
	first := selector(items[0])
	for _, item := range items[1:] {
		if !reflect.DeepEqual(first, selector(item)) {
			return false
		}
	}
	return true
}

func allEqualRP(items []*model.RoutingPolicy) bool {
	if len(items) < 2 {
		return true
	}
	for _, item := range items[1:] {
		if !reflect.DeepEqual(items[0], item) {
			return false
		}
	}
	return true
}

// subtract removes config in b from a, returning the remainder
func subtract(a, b *model.DeviceModel) *model.DeviceModel {
	if a == nil {
		return nil
	}
	if b == nil {
		return a
	}

	result := deepCopy(a)

	// Subtract System
	if result.System != nil && b.System != nil {
		result.System = subtractSystem(result.System, b.System)
	}

	// Subtract BGP
	if result.BGP != nil && b.BGP != nil {
		result.BGP = subtractBGP(result.BGP, b.BGP)
	}

	// Subtract RoutingPolicy
	if result.RoutingPolicy != nil && b.RoutingPolicy != nil {
		if reflect.DeepEqual(result.RoutingPolicy, b.RoutingPolicy) {
			result.RoutingPolicy = nil
		}
	}

	return result
}

func subtractSystem(a, b *model.System) *model.System {
	result := &model.System{
		Hostname:   a.Hostname,
		DomainName: a.DomainName,
	}

	// Keep NTP only if different from common
	if a.NTP != nil && !reflect.DeepEqual(a.NTP, b.NTP) {
		result.NTP = a.NTP
	}

	// Keep DNS only if different
	if a.DNS != nil && !reflect.DeepEqual(a.DNS, b.DNS) {
		result.DNS = a.DNS
	}

	// Keep AAA only if different
	if a.AAA != nil && !reflect.DeepEqual(a.AAA, b.AAA) {
		result.AAA = a.AAA
	}

	// Keep Logging only if different
	if a.Logging != nil && !reflect.DeepEqual(a.Logging, b.Logging) {
		result.Logging = a.Logging
	}

	if isEmpty(result) {
		return nil
	}
	return result
}

func subtractBGP(a, b *model.BGP) *model.BGP {
	result := &model.BGP{}

	// Keep Global only if different
	if !reflect.DeepEqual(a.Global, b.Global) {
		result.Global = a.Global
	}

	// Keep peer groups not in common (or different from common)
	if a.PeerGroups != nil {
		remaining := make(map[string]*model.BGPPeerGroup)
		for name, pg := range a.PeerGroups {
			if b.PeerGroups == nil {
				remaining[name] = pg
			} else if commonPG, ok := b.PeerGroups[name]; !ok || !reflect.DeepEqual(pg, commonPG) {
				remaining[name] = pg
			}
		}
		if len(remaining) > 0 {
			result.PeerGroups = remaining
		}
	}

	// Keep all neighbors (they're device-specific)
	result.Neighbors = a.Neighbors

	if isEmpty(result) {
		return nil
	}
	return result
}

// Deep copy helpers
func deepCopy(dm *model.DeviceModel) *model.DeviceModel {
	if dm == nil {
		return nil
	}
	result := &model.DeviceModel{
		Metadata: dm.Metadata,
	}
	if dm.Interfaces != nil {
		result.Interfaces = make(map[string]*model.Interface)
		for k, v := range dm.Interfaces {
			result.Interfaces[k] = v // Shallow for now, interfaces stay in host_vars
		}
	}
	if dm.BGP != nil {
		result.BGP = deepCopyBGP(dm.BGP)
	}
	if dm.OSPF != nil {
		result.OSPF = dm.OSPF // Shallow, stays in host_vars
	}
	if dm.EVPN != nil {
		result.EVPN = dm.EVPN // Shallow, stays in host_vars
	}
	if dm.System != nil {
		result.System = deepCopySystem(dm.System)
	}
	if dm.RoutingPolicy != nil {
		result.RoutingPolicy = deepCopyRoutingPolicy(dm.RoutingPolicy)
	}
	return result
}

func deepCopySystem(s *model.System) *model.System {
	if s == nil {
		return nil
	}
	result := &model.System{
		Hostname:   s.Hostname,
		DomainName: s.DomainName,
	}
	if s.NTP != nil {
		result.NTP = deepCopyNTP(s.NTP)
	}
	if s.DNS != nil {
		result.DNS = deepCopyDNS(s.DNS)
	}
	if s.AAA != nil {
		result.AAA = deepCopyAAA(s.AAA)
	}
	if s.Logging != nil {
		result.Logging = deepCopyLogging(s.Logging)
	}
	return result
}

func deepCopyNTP(n *model.NTP) *model.NTP {
	if n == nil {
		return nil
	}
	result := &model.NTP{Enabled: n.Enabled}
	if n.Servers != nil {
		result.Servers = make([]model.NTPServer, len(n.Servers))
		copy(result.Servers, n.Servers)
	}
	return result
}

func deepCopyDNS(d *model.DNS) *model.DNS {
	if d == nil {
		return nil
	}
	result := &model.DNS{}
	if d.Servers != nil {
		result.Servers = make([]string, len(d.Servers))
		copy(result.Servers, d.Servers)
	}
	if d.Search != nil {
		result.Search = make([]string, len(d.Search))
		copy(result.Search, d.Search)
	}
	return result
}

func deepCopyAAA(a *model.AAA) *model.AAA {
	if a == nil {
		return nil
	}
	result := &model.AAA{}
	if a.Users != nil {
		result.Users = make([]model.User, len(a.Users))
		copy(result.Users, a.Users)
	}
	return result
}

func deepCopyLogging(l *model.Logging) *model.Logging {
	if l == nil {
		return nil
	}
	result := &model.Logging{}
	if l.Servers != nil {
		result.Servers = make([]model.LogServer, len(l.Servers))
		copy(result.Servers, l.Servers)
	}
	return result
}

func deepCopyBGP(b *model.BGP) *model.BGP {
	if b == nil {
		return nil
	}
	result := &model.BGP{
		Global: b.Global,
	}
	if b.PeerGroups != nil {
		result.PeerGroups = make(map[string]*model.BGPPeerGroup)
		for k, v := range b.PeerGroups {
			result.PeerGroups[k] = deepCopyPeerGroup(v)
		}
	}
	if b.Neighbors != nil {
		result.Neighbors = make(map[string]*model.BGPNeighbor)
		for k, v := range b.Neighbors {
			result.Neighbors[k] = v // Shallow copy for neighbors
		}
	}
	return result
}

func deepCopyPeerGroup(pg *model.BGPPeerGroup) *model.BGPPeerGroup {
	if pg == nil {
		return nil
	}
	result := *pg // Struct copy
	if pg.AFI != nil {
		result.AFI = make([]model.BGPAfiSafi, len(pg.AFI))
		copy(result.AFI, pg.AFI)
	}
	if pg.Timers != nil {
		t := *pg.Timers
		result.Timers = &t
	}
	return &result
}

func deepCopyRoutingPolicy(rp *model.RoutingPolicy) *model.RoutingPolicy {
	if rp == nil {
		return nil
	}
	result := &model.RoutingPolicy{}
	if rp.DefinedSets != nil {
		ds := &model.DefinedSets{}
		if rp.DefinedSets.PrefixSets != nil {
			ds.PrefixSets = make([]model.PrefixSet, len(rp.DefinedSets.PrefixSets))
			copy(ds.PrefixSets, rp.DefinedSets.PrefixSets)
		}
		if rp.DefinedSets.CommunitySets != nil {
			ds.CommunitySets = make([]model.CommunitySet, len(rp.DefinedSets.CommunitySets))
			copy(ds.CommunitySets, rp.DefinedSets.CommunitySets)
		}
		if rp.DefinedSets.ASPathSets != nil {
			ds.ASPathSets = make([]model.ASPathSet, len(rp.DefinedSets.ASPathSets))
			copy(ds.ASPathSets, rp.DefinedSets.ASPathSets)
		}
		result.DefinedSets = ds
	}
	if rp.PolicyDefinitions != nil {
		result.PolicyDefinitions = make([]model.PolicyDefinition, len(rp.PolicyDefinitions))
		copy(result.PolicyDefinitions, rp.PolicyDefinitions)
	}
	return result
}

// isEmpty checks if a model/struct has no meaningful data
func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}

	switch m := v.(type) {
	case *model.DeviceModel:
		return m.Interfaces == nil && m.BGP == nil && m.OSPF == nil &&
			m.EVPN == nil && m.System == nil && m.RoutingPolicy == nil
	case *model.System:
		return m.Hostname == "" && m.DomainName == "" &&
			m.NTP == nil && m.DNS == nil && m.AAA == nil && m.Logging == nil
	case *model.BGP:
		return reflect.DeepEqual(m.Global, model.BGPGlobal{}) &&
			len(m.PeerGroups) == 0 && len(m.Neighbors) == 0
	default:
		return reflect.ValueOf(v).IsZero()
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
