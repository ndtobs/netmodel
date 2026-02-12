package exporter

import (
	"context"
	"strings"

	"github.com/ndtobs/netmodel/internal/gnmi"
	"github.com/ndtobs/netmodel/internal/model"
)

// OSPFExporter exports OSPF configuration
type OSPFExporter struct {
	ospf *model.OSPF
}

func (e *OSPFExporter) Name() string {
	return "ospf"
}

func (e *OSPFExporter) Export(ctx context.Context, client *gnmi.Client) error {
	e.ospf = &model.OSPF{
		Global: &model.OSPFGlobal{},
		Areas:  make(map[string]*model.OSPFArea),
	}

	// Get OSPF config
	ospfData, err := client.GetJSON(ctx, "/network-instances/network-instance[name=default]/protocols/protocol[identifier=OSPF][name=OSPF]/ospf")
	if err != nil {
		// OSPF not configured - that's fine
		if strings.Contains(err.Error(), "NotFound") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "InvalidArgument") {
			return nil
		}
		return err
	}

	if ospfData == nil {
		return nil
	}

	// Parse global config
	if global, ok := ospfData["openconfig-network-instance:global"].(map[string]interface{}); ok {
		e.parseGlobal(global)
	} else if global, ok := ospfData["global"].(map[string]interface{}); ok {
		e.parseGlobal(global)
	}

	// Parse areas
	if areas, ok := ospfData["openconfig-network-instance:areas"].(map[string]interface{}); ok {
		e.parseAreas(areas)
	} else if areas, ok := ospfData["areas"].(map[string]interface{}); ok {
		e.parseAreas(areas)
	}

	return nil
}

func (e *OSPFExporter) parseGlobal(data map[string]interface{}) {
	config := data
	if cfg, ok := data["config"].(map[string]interface{}); ok {
		config = cfg
	}

	if routerID, ok := config["router-id"].(string); ok {
		e.ospf.Global.RouterID = routerID
	}
}

func (e *OSPFExporter) parseAreas(data map[string]interface{}) {
	var areaList []interface{}

	if areas, ok := data["area"].([]interface{}); ok {
		areaList = areas
	} else if areas, ok := data["openconfig-network-instance:area"].([]interface{}); ok {
		areaList = areas
	}

	for _, a := range areaList {
		areaData, ok := a.(map[string]interface{})
		if !ok {
			continue
		}

		identifier, _ := areaData["identifier"].(string)
		if identifier == "" {
			// Try as number
			if id, ok := areaData["identifier"].(float64); ok {
				identifier = formatAreaID(id)
			}
		}
		if identifier == "" {
			continue
		}

		area := &model.OSPFArea{
			Identifier: identifier,
			Interfaces: make(map[string]*model.OSPFInterface),
		}

		// Parse interfaces
		if interfaces, ok := areaData["interfaces"].(map[string]interface{}); ok {
			e.parseInterfaces(area, interfaces)
		} else if interfaces, ok := areaData["openconfig-network-instance:interfaces"].(map[string]interface{}); ok {
			e.parseInterfaces(area, interfaces)
		}

		e.ospf.Areas[identifier] = area
	}
}

func (e *OSPFExporter) parseInterfaces(area *model.OSPFArea, data map[string]interface{}) {
	var ifaceList []interface{}

	if ifaces, ok := data["interface"].([]interface{}); ok {
		ifaceList = ifaces
	} else if ifaces, ok := data["openconfig-network-instance:interface"].([]interface{}); ok {
		ifaceList = ifaces
	}

	for _, i := range ifaceList {
		ifaceData, ok := i.(map[string]interface{})
		if !ok {
			continue
		}

		ifaceID, _ := ifaceData["id"].(string)
		if ifaceID == "" {
			continue
		}

		iface := &model.OSPFInterface{}

		// Parse config
		config, ok := ifaceData["config"].(map[string]interface{})
		if !ok {
			config = ifaceData
		}

		if netType, ok := config["network-type"].(string); ok {
			iface.NetworkType = stripNamespace(netType)
		}

		if passive, ok := config["passive"].(bool); ok {
			iface.Passive = &passive
		}

		if cost, ok := config["metric"].(float64); ok && cost > 0 {
			iface.Cost = int(cost)
		}

		if priority, ok := config["priority"].(float64); ok {
			iface.Priority = int(priority)
		}

		// Parse timers
		if timers, ok := ifaceData["timers"].(map[string]interface{}); ok {
			if tConfig, ok := timers["config"].(map[string]interface{}); ok {
				if hello, ok := tConfig["hello-interval"].(float64); ok && hello > 0 {
					iface.HelloInterval = int(hello)
				}
				if dead, ok := tConfig["dead-interval"].(float64); ok && dead > 0 {
					iface.DeadInterval = int(dead)
				}
			}
		}

		// Only add if we have meaningful config
		if iface.NetworkType != "" || iface.Passive != nil || iface.Cost > 0 ||
			iface.Priority > 0 || iface.HelloInterval > 0 || iface.DeadInterval > 0 {
			area.Interfaces[ifaceID] = iface
		} else {
			// Add with minimal info (just that it's in this area)
			area.Interfaces[ifaceID] = &model.OSPFInterface{}
		}
	}
}

func formatAreaID(id float64) string {
	// Area 0 is typically "0.0.0.0" or just "0"
	if id == 0 {
		return "0.0.0.0"
	}
	return ""
}

func (e *OSPFExporter) Apply(m *model.DeviceModel) {
	// Only apply if we have meaningful OSPF config
	if e.ospf.Global.RouterID != "" || len(e.ospf.Areas) > 0 {
		m.OSPF = e.ospf
	}
}
