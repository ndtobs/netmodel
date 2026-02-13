package exporter

import (
	"context"
	"strings"

	"github.com/ndtobs/netmodel/internal/gnmi"
	"github.com/ndtobs/netmodel/internal/model"
)

// BGPExporter exports BGP configuration
type BGPExporter struct {
	bgp *model.BGP
}

func (e *BGPExporter) Name() string {
	return "bgp"
}

func (e *BGPExporter) Export(ctx context.Context, client *gnmi.Client) error {
	e.bgp = &model.BGP{
		PeerGroups: make(map[string]*model.BGPPeerGroup),
		Neighbors:  make(map[string]*model.BGPNeighbor),
	}

	// Get full BGP config in one query
	bgpData, err := client.GetJSON(ctx, "/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp")
	if err != nil {
		return err
	}

	if bgpData == nil {
		return nil
	}

	// Parse global
	if global, ok := bgpData["openconfig-network-instance:global"].(map[string]interface{}); ok {
		e.parseGlobal(global)
	} else if global, ok := bgpData["global"].(map[string]interface{}); ok {
		e.parseGlobal(global)
	}

	// Parse peer groups
	if peerGroups, ok := bgpData["openconfig-network-instance:peer-groups"].(map[string]interface{}); ok {
		e.parsePeerGroups(peerGroups)
	} else if peerGroups, ok := bgpData["peer-groups"].(map[string]interface{}); ok {
		e.parsePeerGroups(peerGroups)
	}

	// Parse neighbors
	if neighbors, ok := bgpData["openconfig-network-instance:neighbors"].(map[string]interface{}); ok {
		e.parseNeighbors(neighbors)
	} else if neighbors, ok := bgpData["neighbors"].(map[string]interface{}); ok {
		e.parseNeighbors(neighbors)
	}

	return nil
}

func (e *BGPExporter) parseGlobal(data map[string]interface{}) {
	config := data
	if cfg, ok := data["config"].(map[string]interface{}); ok {
		config = cfg
	}

	if as, ok := config["as"].(float64); ok {
		e.bgp.Global.AS = uint32(as)
	}

	if routerID, ok := config["router-id"].(string); ok {
		e.bgp.Global.RouterID = routerID
	}
}

func (e *BGPExporter) parsePeerGroups(data map[string]interface{}) {
	var peerGroupList []interface{}

	if pgs, ok := data["peer-group"].([]interface{}); ok {
		peerGroupList = pgs
	}

	for _, pg := range peerGroupList {
		pgData, ok := pg.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := pgData["peer-group-name"].(string)
		if name == "" {
			continue
		}

		peerGroup := &model.BGPPeerGroup{}

		config, ok := pgData["config"].(map[string]interface{})
		if !ok {
			config = pgData
		}

		if desc, ok := config["description"].(string); ok {
			peerGroup.Description = desc
		}

		if peerAS, ok := config["peer-as"].(float64); ok {
			peerGroup.PeerAS = uint32(peerAS)
		}

		if localAS, ok := config["local-as"].(float64); ok {
			peerGroup.LocalAS = uint32(localAS)
		}

		if peerType, ok := config["peer-type"].(string); ok {
			peerGroup.PeerType = stripNamespace(peerType)
		}

		// Parse timers
		if timers, ok := pgData["timers"].(map[string]interface{}); ok {
			peerGroup.Timers = parseTimers(timers)
		}

		// Parse AFI-SAFI
		if afiSafis, ok := pgData["afi-safis"].(map[string]interface{}); ok {
			peerGroup.AFI = parseAfiSafis(afiSafis)
		}

		// Parse transport for update-source
		if transport, ok := pgData["transport"].(map[string]interface{}); ok {
			if tConfig, ok := transport["config"].(map[string]interface{}); ok {
				if updateSource, ok := tConfig["local-address"].(string); ok {
					peerGroup.UpdateSource = updateSource
				}
			}
		}

		// Parse ebgp-multihop
		if multihop, ok := pgData["ebgp-multihop"].(map[string]interface{}); ok {
			if mConfig, ok := multihop["config"].(map[string]interface{}); ok {
				if ttl, ok := mConfig["multihop-ttl"].(float64); ok && ttl > 0 {
					peerGroup.EBGPMultihop = int(ttl)
				}
			}
		}

		// Route reflector client
		if rrConfig, ok := pgData["route-reflector"].(map[string]interface{}); ok {
			if cfg, ok := rrConfig["config"].(map[string]interface{}); ok {
				if rrClient, ok := cfg["route-reflector-client"].(bool); ok && rrClient {
					peerGroup.RouteReflector = &rrClient
				}
			}
		}

		e.bgp.PeerGroups[name] = peerGroup
	}
}

func (e *BGPExporter) parseNeighbors(data map[string]interface{}) {
	var neighborList []interface{}

	if nbrs, ok := data["neighbor"].([]interface{}); ok {
		neighborList = nbrs
	}

	for _, nbr := range neighborList {
		nbrData, ok := nbr.(map[string]interface{})
		if !ok {
			continue
		}

		neighborAddr, _ := nbrData["neighbor-address"].(string)
		if neighborAddr == "" {
			continue
		}

		neighbor := &model.BGPNeighbor{}

		config, ok := nbrData["config"].(map[string]interface{})
		if !ok {
			config = nbrData
		}

		if desc, ok := config["description"].(string); ok {
			neighbor.Description = desc
		}

		if enabled, ok := config["enabled"].(bool); ok {
			neighbor.Enabled = &enabled
		}

		if peerAS, ok := config["peer-as"].(float64); ok {
			neighbor.PeerAS = uint32(peerAS)
		}

		if peerGroup, ok := config["peer-group"].(string); ok {
			neighbor.PeerGroup = stripNamespace(peerGroup)
		}

		if localAS, ok := config["local-as"].(float64); ok {
			neighbor.LocalAS = uint32(localAS)
		}

		if peerType, ok := config["peer-type"].(string); ok {
			neighbor.PeerType = stripNamespace(peerType)
		}

		if sendComm, ok := config["send-community"].(string); ok {
			neighbor.SendCommunity = stripNamespace(sendComm)
		}

		// Parse timers
		if timers, ok := nbrData["timers"].(map[string]interface{}); ok {
			neighbor.Timers = parseTimers(timers)
		}

		// Parse AFI-SAFI
		if afiSafis, ok := nbrData["afi-safis"].(map[string]interface{}); ok {
			neighbor.AFI = parseAfiSafis(afiSafis)
		}

		// Parse transport for update-source
		if transport, ok := nbrData["transport"].(map[string]interface{}); ok {
			if tConfig, ok := transport["config"].(map[string]interface{}); ok {
				if updateSource, ok := tConfig["local-address"].(string); ok {
					neighbor.UpdateSource = updateSource
				}
			}
		}

		// Parse ebgp-multihop
		if multihop, ok := nbrData["ebgp-multihop"].(map[string]interface{}); ok {
			if mConfig, ok := multihop["config"].(map[string]interface{}); ok {
				if ttl, ok := mConfig["multihop-ttl"].(float64); ok && ttl > 0 {
					neighbor.EBGPMultihop = int(ttl)
				}
			}
		}

		// Parse apply-policy for import/export
		if applyPolicy, ok := nbrData["apply-policy"].(map[string]interface{}); ok {
			if pConfig, ok := applyPolicy["config"].(map[string]interface{}); ok {
				if importPolicy, ok := pConfig["import-policy"].([]interface{}); ok && len(importPolicy) > 0 {
					if p, ok := importPolicy[0].(string); ok {
						neighbor.ImportPolicy = p
					}
				}
				if exportPolicy, ok := pConfig["export-policy"].([]interface{}); ok && len(exportPolicy) > 0 {
					if p, ok := exportPolicy[0].(string); ok {
						neighbor.ExportPolicy = p
					}
				}
			}
		}

		// Route reflector client
		if rrConfig, ok := nbrData["route-reflector"].(map[string]interface{}); ok {
			if cfg, ok := rrConfig["config"].(map[string]interface{}); ok {
				if rrClient, ok := cfg["route-reflector-client"].(bool); ok && rrClient {
					neighbor.RouteReflector = &rrClient
				}
			}
		}

		e.bgp.Neighbors[neighborAddr] = neighbor
	}
}

func parseTimers(timers map[string]interface{}) *model.BGPTimers {
	config, ok := timers["config"].(map[string]interface{})
	if !ok {
		return nil
	}

	t := &model.BGPTimers{}

	if hold, ok := config["hold-time"].(float64); ok && hold > 0 {
		t.HoldTime = int(hold)
	}

	if keepalive, ok := config["keepalive-interval"].(float64); ok && keepalive > 0 {
		t.KeepaliveTime = int(keepalive)
	}

	if connect, ok := config["connect-retry"].(float64); ok && connect > 0 {
		t.ConnectRetry = int(connect)
	}

	// Only return if we got something
	if t.HoldTime == 0 && t.KeepaliveTime == 0 && t.ConnectRetry == 0 {
		return nil
	}

	return t
}

func parseAfiSafis(afiSafis map[string]interface{}) []model.BGPAfiSafi {
	var result []model.BGPAfiSafi

	afiList, ok := afiSafis["afi-safi"].([]interface{})
	if !ok {
		return nil
	}

	for _, afi := range afiList {
		afiData, ok := afi.(map[string]interface{})
		if !ok {
			continue
		}

		afiSafi := model.BGPAfiSafi{}
		enabled := false

		// Get AFI name
		if name, ok := afiData["afi-safi-name"].(string); ok {
			afiSafi.Name = stripNamespace(name)
		}

		// Check if active/enabled from state (config doesn't always have enabled)
		if state, ok := afiData["state"].(map[string]interface{}); ok {
			if active, ok := state["active"].(bool); ok {
				enabled = active
			}
		}

		// Get config if present (overrides state)
		if config, ok := afiData["config"].(map[string]interface{}); ok {
			if e, ok := config["enabled"].(bool); ok {
				enabled = e
			}
		}

		// Only include enabled AFI-SAFIs
		if afiSafi.Name != "" && isCommonAfi(afiSafi.Name) && enabled {
			// Don't include enabled field in output since they're all enabled
			result = append(result, afiSafi)
		}
	}

	return result
}

func isCommonAfi(name string) bool {
	common := map[string]bool{
		"IPV4_UNICAST":   true,
		"IPV6_UNICAST":   true,
		"L2VPN_EVPN":     true,
		"IPV4_MULTICAST": true,
		"IPV6_MULTICAST": true,
	}
	return common[name]
}

func stripNamespace(s string) string {
	if idx := strings.LastIndex(s, ":"); idx != -1 {
		return s[idx+1:]
	}
	return s
}

func (e *BGPExporter) Apply(m *model.DeviceModel) {
	// Clean up redundant neighbor AFIs that are inherited from peer groups
	e.cleanupInheritedAFI()

	if e.bgp.Global.AS > 0 || len(e.bgp.Neighbors) > 0 || len(e.bgp.PeerGroups) > 0 {
		m.BGP = e.bgp
	}
}

// cleanupInheritedAFI removes AFI-SAFI from neighbors when it matches their peer group
func (e *BGPExporter) cleanupInheritedAFI() {
	for _, neighbor := range e.bgp.Neighbors {
		if neighbor.PeerGroup == "" || len(neighbor.AFI) == 0 {
			continue
		}

		// Find the peer group
		pg, ok := e.bgp.PeerGroups[neighbor.PeerGroup]
		if !ok || len(pg.AFI) == 0 {
			continue
		}

		// Check if neighbor AFI matches peer group AFI
		if afiListsEqual(neighbor.AFI, pg.AFI) {
			neighbor.AFI = nil // Remove redundant AFI
		}
	}
}

func afiListsEqual(a, b []model.BGPAfiSafi) bool {
	if len(a) != len(b) {
		return false
	}

	// Build set from b
	bSet := make(map[string]bool)
	for _, afi := range b {
		bSet[afi.Name] = true
	}

	// Check all of a are in b
	for _, afi := range a {
		if !bSet[afi.Name] {
			return false
		}
	}

	return true
}
