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

	// Get BGP global config
	// Path: /network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/global/config
	globalData, err := client.GetJSON(ctx, "/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/global")
	if err == nil && globalData != nil {
		e.parseGlobal(globalData)
	}

	// Get peer groups
	peerGroupData, err := client.GetJSON(ctx, "/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/peer-groups")
	if err == nil && peerGroupData != nil {
		e.parsePeerGroups(peerGroupData)
	}

	// Get neighbors
	neighborsData, err := client.GetJSON(ctx, "/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors")
	if err == nil && neighborsData != nil {
		e.parseNeighbors(neighborsData)
	}

	return nil
}

func (e *BGPExporter) parseGlobal(data map[string]interface{}) {
	// Handle namespaced keys
	config := data
	if cfg, ok := data["config"].(map[string]interface{}); ok {
		config = cfg
	}
	if cfg, ok := data["openconfig-network-instance:config"].(map[string]interface{}); ok {
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
	// Handle different response structures
	var peerGroupList []interface{}

	if pgs, ok := data["openconfig-network-instance:peer-group"].([]interface{}); ok {
		peerGroupList = pgs
	} else if pgs, ok := data["peer-group"].([]interface{}); ok {
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

		e.bgp.PeerGroups[name] = peerGroup
	}
}

func (e *BGPExporter) parseNeighbors(data map[string]interface{}) {
	var neighborList []interface{}

	if nbrs, ok := data["openconfig-network-instance:neighbor"].([]interface{}); ok {
		neighborList = nbrs
	} else if nbrs, ok := data["neighbor"].([]interface{}); ok {
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
			// Strip namespace prefix if present
			if idx := strings.LastIndex(peerGroup, ":"); idx != -1 {
				peerGroup = peerGroup[idx+1:]
			}
			neighbor.PeerGroup = peerGroup
		}

		if localAS, ok := config["local-as"].(float64); ok {
			neighbor.LocalAS = uint32(localAS)
		}

		e.bgp.Neighbors[neighborAddr] = neighbor
	}
}

func (e *BGPExporter) Apply(m *model.DeviceModel) {
	// Only set BGP if we got meaningful data
	if e.bgp.Global.AS > 0 || len(e.bgp.Neighbors) > 0 || len(e.bgp.PeerGroups) > 0 {
		m.BGP = e.bgp
	}
}
