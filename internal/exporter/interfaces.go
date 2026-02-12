package exporter

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ndtobs/netmodel/internal/gnmi"
	"github.com/ndtobs/netmodel/internal/model"
)

// InterfacesExporter exports interface configuration
type InterfacesExporter struct {
	interfaces map[string]*model.Interface
}

func (e *InterfacesExporter) Name() string {
	return "interfaces"
}

func (e *InterfacesExporter) Export(ctx context.Context, client *gnmi.Client) error {
	e.interfaces = make(map[string]*model.Interface)

	// Get all interfaces config
	data, err := client.GetJSON(ctx, "/interfaces/interface/config")
	if err != nil {
		return err
	}

	if data == nil {
		return nil
	}

	// Parse the response - handle both single interface and array
	if err := e.parseInterfacesConfig(data); err != nil {
		return err
	}

	// Get subinterfaces for IP addresses
	subintData, err := client.GetJSON(ctx, "/interfaces/interface/subinterfaces")
	if err == nil && subintData != nil {
		e.parseSubinterfaces(subintData)
	}

	return nil
}

func (e *InterfacesExporter) parseInterfacesConfig(data map[string]interface{}) error {
	// The response structure varies by vendor
	// Try to handle common patterns

	// Check for "openconfig-interfaces:interface" (list)
	if ifaces, ok := data["openconfig-interfaces:interface"]; ok {
		return e.parseInterfaceList(ifaces)
	}

	// Check for "interface" (list)
	if ifaces, ok := data["interface"]; ok {
		return e.parseInterfaceList(ifaces)
	}

	// Maybe it's a direct config response
	if name, ok := data["name"].(string); ok {
		e.parseInterfaceConfig(name, data)
		return nil
	}

	return nil
}

func (e *InterfacesExporter) parseInterfaceList(ifaces interface{}) error {
	list, ok := ifaces.([]interface{})
	if !ok {
		return nil
	}

	for _, item := range list {
		ifaceData, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := ifaceData["name"].(string)
		if name == "" {
			continue
		}

		// Get config sub-object
		config, ok := ifaceData["config"].(map[string]interface{})
		if ok {
			e.parseInterfaceConfig(name, config)
		} else {
			// Config might be at top level
			e.parseInterfaceConfig(name, ifaceData)
		}

		// Parse subinterfaces if present
		if subints, ok := ifaceData["subinterfaces"].(map[string]interface{}); ok {
			e.parseInterfaceSubints(name, subints)
		}
	}

	return nil
}

func (e *InterfacesExporter) parseInterfaceConfig(name string, config map[string]interface{}) {
	iface := &model.Interface{}

	if desc, ok := config["description"].(string); ok {
		iface.Description = desc
	}

	if enabled, ok := config["enabled"].(bool); ok {
		iface.Enabled = &enabled
	}

	if mtu, ok := config["mtu"].(float64); ok {
		iface.MTU = int(mtu)
	}

	if ifType, ok := config["type"].(string); ok {
		// Strip namespace prefix
		if idx := strings.LastIndex(ifType, ":"); idx != -1 {
			ifType = ifType[idx+1:]
		}
		iface.Type = ifType
	}

	// Only add if there's meaningful config
	if iface.Description != "" || iface.Enabled != nil || iface.MTU > 0 {
		e.interfaces[name] = iface
	}
}

func (e *InterfacesExporter) parseSubinterfaces(data map[string]interface{}) {
	// Handle various response structures
	if ifaces, ok := data["openconfig-interfaces:interface"]; ok {
		list, ok := ifaces.([]interface{})
		if !ok {
			return
		}
		for _, item := range list {
			ifaceData, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := ifaceData["name"].(string)
			if subints, ok := ifaceData["subinterfaces"].(map[string]interface{}); ok {
				e.parseInterfaceSubints(name, subints)
			}
		}
	}
}

func (e *InterfacesExporter) parseInterfaceSubints(ifaceName string, subints map[string]interface{}) {
	// Get or create interface
	iface, ok := e.interfaces[ifaceName]
	if !ok {
		iface = &model.Interface{}
		e.interfaces[ifaceName] = iface
	}

	// Parse subinterface list
	subintList, ok := subints["subinterface"].([]interface{})
	if !ok {
		return
	}

	for _, subint := range subintList {
		subintData, ok := subint.(map[string]interface{})
		if !ok {
			continue
		}

		// Get IPv4 addresses
		if ipv4, ok := subintData["openconfig-if-ip:ipv4"].(map[string]interface{}); ok {
			e.parseIPv4(iface, ipv4)
		} else if ipv4, ok := subintData["ipv4"].(map[string]interface{}); ok {
			e.parseIPv4(iface, ipv4)
		}

		// Get IPv6 addresses
		if ipv6, ok := subintData["openconfig-if-ip:ipv6"].(map[string]interface{}); ok {
			e.parseIPv6(iface, ipv6)
		} else if ipv6, ok := subintData["ipv6"].(map[string]interface{}); ok {
			e.parseIPv6(iface, ipv6)
		}
	}
}

func (e *InterfacesExporter) parseIPv4(iface *model.Interface, ipv4 map[string]interface{}) {
	addresses, ok := ipv4["addresses"].(map[string]interface{})
	if !ok {
		return
	}

	addrList, ok := addresses["address"].([]interface{})
	if !ok {
		return
	}

	for _, addr := range addrList {
		addrData, ok := addr.(map[string]interface{})
		if !ok {
			continue
		}

		config, ok := addrData["config"].(map[string]interface{})
		if !ok {
			config = addrData // Try top level
		}

		ip, _ := config["ip"].(string)
		prefixLen, _ := config["prefix-length"].(float64)

		if ip != "" {
			if iface.IPv4 == nil {
				iface.IPv4 = &model.InterfaceIPv4{}
			}
			iface.IPv4.Addresses = append(iface.IPv4.Addresses, model.IPv4Address{
				IP:           ip,
				PrefixLength: int(prefixLen),
			})
		}
	}
}

func (e *InterfacesExporter) parseIPv6(iface *model.Interface, ipv6 map[string]interface{}) {
	addresses, ok := ipv6["addresses"].(map[string]interface{})
	if !ok {
		return
	}

	addrList, ok := addresses["address"].([]interface{})
	if !ok {
		return
	}

	for _, addr := range addrList {
		addrData, ok := addr.(map[string]interface{})
		if !ok {
			continue
		}

		config, ok := addrData["config"].(map[string]interface{})
		if !ok {
			config = addrData
		}

		ip, _ := config["ip"].(string)
		prefixLen, _ := config["prefix-length"].(float64)

		if ip != "" {
			if iface.IPv6 == nil {
				iface.IPv6 = &model.InterfaceIPv6{}
			}
			iface.IPv6.Addresses = append(iface.IPv6.Addresses, model.IPv6Address{
				IP:           ip,
				PrefixLength: int(prefixLen),
			})
		}
	}
}

func (e *InterfacesExporter) Apply(m *model.DeviceModel) {
	if len(e.interfaces) > 0 {
		m.Interfaces = e.interfaces
	}
}

// Helper for debugging
func toJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
