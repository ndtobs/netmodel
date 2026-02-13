package dedup

import (
	"testing"

	"github.com/ndtobs/netmodel/internal/model"
)

func TestDeduplicateCommonNTP(t *testing.T) {
	// All devices have identical NTP config
	models := map[string]*model.DeviceModel{
		"spine1": {
			System: &model.System{
				Hostname: "spine1",
				NTP: &model.NTP{
					Enabled: true,
					Servers: []model.NTPServer{
						{Address: "10.0.0.1", Prefer: true},
						{Address: "10.0.0.2"},
					},
				},
			},
		},
		"spine2": {
			System: &model.System{
				Hostname: "spine2",
				NTP: &model.NTP{
					Enabled: true,
					Servers: []model.NTPServer{
						{Address: "10.0.0.1", Prefer: true},
						{Address: "10.0.0.2"},
					},
				},
			},
		},
	}

	result := Deduplicate(models, nil)

	// Common should have NTP
	if result.Common.System == nil || result.Common.System.NTP == nil {
		t.Fatal("expected common NTP config")
	}
	if len(result.Common.System.NTP.Servers) != 2 {
		t.Errorf("expected 2 NTP servers in common, got %d", len(result.Common.System.NTP.Servers))
	}

	// Host-specific should NOT have NTP (it's in common)
	for host, specific := range result.HostSpecific {
		if specific.System != nil && specific.System.NTP != nil {
			t.Errorf("%s should not have NTP in host-specific (it's common)", host)
		}
	}
}

func TestDeduplicateDifferentNTP(t *testing.T) {
	// Devices have different NTP config
	models := map[string]*model.DeviceModel{
		"spine1": {
			System: &model.System{
				Hostname: "spine1",
				NTP: &model.NTP{
					Servers: []model.NTPServer{{Address: "10.0.0.1"}},
				},
			},
		},
		"spine2": {
			System: &model.System{
				Hostname: "spine2",
				NTP: &model.NTP{
					Servers: []model.NTPServer{{Address: "10.0.0.99"}}, // Different!
				},
			},
		},
	}

	result := Deduplicate(models, nil)

	// Common should NOT have NTP (configs differ)
	if result.Common.System != nil && result.Common.System.NTP != nil {
		t.Error("expected no common NTP config (configs differ)")
	}

	// Each host should keep their NTP
	for host, specific := range result.HostSpecific {
		if specific.System == nil || specific.System.NTP == nil {
			t.Errorf("%s should have NTP in host-specific", host)
		}
	}
}

func TestDeduplicateCommonPeerGroups(t *testing.T) {
	// All devices have identical peer group config
	pg := &model.BGPPeerGroup{
		PeerAS:        65100,
		SendCommunity: "BOTH",
	}

	models := map[string]*model.DeviceModel{
		"spine1": {
			BGP: &model.BGP{
				Global:     model.BGPGlobal{AS: 65001, RouterID: "10.255.0.1"}, // Different router_id
				PeerGroups: map[string]*model.BGPPeerGroup{"LEAF": pg},
				Neighbors:  map[string]*model.BGPNeighbor{"10.0.0.1": {PeerAS: 65101}},
			},
		},
		"spine2": {
			BGP: &model.BGP{
				Global:     model.BGPGlobal{AS: 65001, RouterID: "10.255.0.2"}, // Different router_id
				PeerGroups: map[string]*model.BGPPeerGroup{"LEAF": pg},
				Neighbors:  map[string]*model.BGPNeighbor{"10.0.0.2": {PeerAS: 65102}},
			},
		},
	}

	result := Deduplicate(models, nil)

	// Common should have LEAF peer group
	if result.Common.BGP == nil || result.Common.BGP.PeerGroups == nil {
		t.Fatal("expected common BGP peer groups")
	}
	if _, ok := result.Common.BGP.PeerGroups["LEAF"]; !ok {
		t.Error("expected LEAF peer group in common")
	}

	// Common should NOT have Global (router_id differs)
	if result.Common.BGP.Global.RouterID != "" {
		t.Error("expected no common router_id (they differ)")
	}

	// Host-specific should have neighbors but NOT peer groups
	for host, specific := range result.HostSpecific {
		if specific.BGP == nil {
			t.Errorf("%s should have BGP in host-specific", host)
			continue
		}
		if specific.BGP.Neighbors == nil || len(specific.BGP.Neighbors) == 0 {
			t.Errorf("%s should have neighbors in host-specific", host)
		}
		if specific.BGP.PeerGroups != nil && len(specific.BGP.PeerGroups) > 0 {
			t.Errorf("%s should not have LEAF peer group in host-specific (it's common)", host)
		}
	}
}

func TestDeduplicateGroupCommon(t *testing.T) {
	// Spines have one config, leaves have another
	spinePG := &model.BGPPeerGroup{PeerAS: 65100, Description: "To Leaves"}
	leafPG := &model.BGPPeerGroup{PeerAS: 65001, Description: "To Spines"}

	models := map[string]*model.DeviceModel{
		"spine1": {BGP: &model.BGP{PeerGroups: map[string]*model.BGPPeerGroup{"DOWNLINK": spinePG}}},
		"spine2": {BGP: &model.BGP{PeerGroups: map[string]*model.BGPPeerGroup{"DOWNLINK": spinePG}}},
		"leaf1":  {BGP: &model.BGP{PeerGroups: map[string]*model.BGPPeerGroup{"UPLINK": leafPG}}},
		"leaf2":  {BGP: &model.BGP{PeerGroups: map[string]*model.BGPPeerGroup{"UPLINK": leafPG}}},
	}

	groups := map[string][]string{
		"spine": {"spine1", "spine2"},
		"leaf":  {"leaf1", "leaf2"},
	}

	result := Deduplicate(models, groups)

	// No global common (spines and leaves have different configs)
	if result.Common.BGP != nil && len(result.Common.BGP.PeerGroups) > 0 {
		t.Error("expected no global common peer groups")
	}

	// Spine group should have DOWNLINK
	if spineCommon, ok := result.GroupCommon["spine"]; ok {
		if spineCommon.BGP == nil || spineCommon.BGP.PeerGroups == nil {
			t.Error("expected spine group to have BGP peer groups")
		} else if _, ok := spineCommon.BGP.PeerGroups["DOWNLINK"]; !ok {
			t.Error("expected DOWNLINK in spine group common")
		}
	} else {
		t.Error("expected spine group common config")
	}

	// Leaf group should have UPLINK
	if leafCommon, ok := result.GroupCommon["leaf"]; ok {
		if leafCommon.BGP == nil || leafCommon.BGP.PeerGroups == nil {
			t.Error("expected leaf group to have BGP peer groups")
		} else if _, ok := leafCommon.BGP.PeerGroups["UPLINK"]; !ok {
			t.Error("expected UPLINK in leaf group common")
		}
	} else {
		t.Error("expected leaf group common config")
	}

	// Host-specific should have no peer groups (they're in group common)
	for host, specific := range result.HostSpecific {
		if specific.BGP != nil && len(specific.BGP.PeerGroups) > 0 {
			t.Errorf("%s should not have peer groups in host-specific", host)
		}
	}
}
