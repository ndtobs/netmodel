package exporter

import (
	"context"
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

	// Get all interfaces - Arista returns at /interfaces
	data, err := client.GetJSON(ctx, "/interfaces")
	if err != nil {
		return err
	}

	if data == nil {
		return nil
	}

	// Parse the interface list
	e.parseInterfaces(data)

	return nil
}

func (e *InterfacesExporter) parseInterfaces(data map[string]interface{}) {
	// Arista returns: {"openconfig-interfaces:interface": [...]}
	var interfaceList []interface{}

	if ifaces, ok := data["openconfig-interfaces:interface"].([]interface{}); ok {
		interfaceList = ifaces
	} else if ifaces, ok := data["interface"].([]interface{}); ok {
		interfaceList = ifaces
	}

	for _, item := range interfaceList {
		ifaceData, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Get interface name
		name, _ := ifaceData["name"].(string)
		if name == "" {
			continue
		}

		iface := &model.Interface{}

		// Parse config block
		if config, ok := ifaceData["config"].(map[string]interface{}); ok {
			e.parseConfig(iface, config)
		}

		// Parse subinterfaces for IP addresses
		if subints, ok := ifaceData["subinterfaces"].(map[string]interface{}); ok {
			e.parseSubinterfaces(iface, subints)
		}

		// Only add if there's meaningful config
		if iface.Description != "" || iface.Enabled != nil || iface.MTU > 0 || iface.IPv4 != nil || iface.IPv6 != nil {
			e.interfaces[name] = iface
		}
	}
}

func (e *InterfacesExporter) parseConfig(iface *model.Interface, config map[string]interface{}) {
	if desc, ok := config["description"].(string); ok {
		iface.Description = desc
	}

	if enabled, ok := config["enabled"].(bool); ok {
		iface.Enabled = &enabled
	}

	if mtu, ok := config["mtu"].(float64); ok && mtu > 0 {
		iface.MTU = int(mtu)
	}

	if ifType, ok := config["type"].(string); ok {
		// Strip namespace prefix (iana-if-type:ethernetCsmacd -> ethernetCsmacd)
		if idx := strings.LastIndex(ifType, ":"); idx != -1 {
			ifType = ifType[idx+1:]
		}
		iface.Type = ifType
	}
}

func (e *InterfacesExporter) parseSubinterfaces(iface *model.Interface, subints map[string]interface{}) {
	// Get subinterface list
	var subintList []interface{}

	if subs, ok := subints["subinterface"].([]interface{}); ok {
		subintList = subs
	}

	for _, sub := range subintList {
		subData, ok := sub.(map[string]interface{})
		if !ok {
			continue
		}

		// Parse IPv4 - check both namespaced and non-namespaced keys
		if ipv4, ok := subData["openconfig-if-ip:ipv4"].(map[string]interface{}); ok {
			e.parseIPv4(iface, ipv4)
		} else if ipv4, ok := subData["ipv4"].(map[string]interface{}); ok {
			e.parseIPv4(iface, ipv4)
		}

		// Parse IPv6
		if ipv6, ok := subData["openconfig-if-ip:ipv6"].(map[string]interface{}); ok {
			e.parseIPv6(iface, ipv6)
		} else if ipv6, ok := subData["ipv6"].(map[string]interface{}); ok {
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

		// Get config (preferred) or top-level
		config, ok := addrData["config"].(map[string]interface{})
		if !ok {
			config = addrData
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

		// Skip link-local addresses
		if strings.HasPrefix(ip, "fe80:") {
			continue
		}

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
