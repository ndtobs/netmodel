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

	// Get all interfaces
	data, err := client.GetJSON(ctx, "/interfaces")
	if err != nil {
		return err
	}

	if data == nil {
		return nil
	}

	e.parseInterfaces(data)

	return nil
}

func (e *InterfacesExporter) parseInterfaces(data map[string]interface{}) {
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

		name, _ := ifaceData["name"].(string)
		if name == "" {
			continue
		}

		iface := &model.Interface{}

		// Parse config block
		if config, ok := ifaceData["config"].(map[string]interface{}); ok {
			e.parseConfig(iface, config)
		}

		// Parse ethernet-specific config
		if eth, ok := ifaceData["openconfig-if-ethernet:ethernet"].(map[string]interface{}); ok {
			e.parseEthernet(iface, eth)
		} else if eth, ok := ifaceData["ethernet"].(map[string]interface{}); ok {
			e.parseEthernet(iface, eth)
		}

		// Parse aggregation (LAG) config
		if agg, ok := ifaceData["openconfig-if-aggregate:aggregation"].(map[string]interface{}); ok {
			e.parseAggregation(iface, agg)
		} else if agg, ok := ifaceData["aggregation"].(map[string]interface{}); ok {
			e.parseAggregation(iface, agg)
		}

		// Parse VXLAN config (Arista extension)
		if vxlan, ok := ifaceData["arista-exp-eos-vxlan:arista-vxlan"].(map[string]interface{}); ok {
			e.parseVXLAN(iface, vxlan)
		} else if vxlan, ok := ifaceData["arista-vxlan"].(map[string]interface{}); ok {
			e.parseVXLAN(iface, vxlan)
		}

		// Parse subinterfaces for IP addresses
		if subints, ok := ifaceData["subinterfaces"].(map[string]interface{}); ok {
			e.parseSubinterfaces(iface, subints)
		}

		// Only add if there's meaningful config
		if e.hasMeaningfulConfig(iface) {
			e.interfaces[name] = iface
		}
	}
}

func (e *InterfacesExporter) hasMeaningfulConfig(iface *model.Interface) bool {
	return iface.Description != "" ||
		iface.Enabled != nil ||
		iface.MTU > 0 ||
		iface.IPv4 != nil ||
		iface.IPv6 != nil ||
		iface.Ethernet != nil ||
		iface.LAG != nil ||
		iface.VXLAN != nil
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
		iface.Type = stripNamespace(ifType)
	}
}

func (e *InterfacesExporter) parseEthernet(iface *model.Interface, eth map[string]interface{}) {
	ethConfig := &model.EthernetConfig{}
	hasConfig := false

	// Check config block
	if config, ok := eth["config"].(map[string]interface{}); ok {
		if speed, ok := config["port-speed"].(string); ok && speed != "SPEED_UNKNOWN" {
			ethConfig.PortSpeed = stripNamespace(speed)
			hasConfig = true
		}

		if autoNeg, ok := config["auto-negotiate"].(bool); ok {
			ethConfig.AutoNegotiate = &autoNeg
			hasConfig = true
		}

		if mac, ok := config["mac-address"].(string); ok && mac != "00:00:00:00:00:00" {
			ethConfig.MacAddress = mac
			hasConfig = true
		}
	}

	// Check state for operational values if config missing
	if state, ok := eth["state"].(map[string]interface{}); ok {
		if ethConfig.PortSpeed == "" {
			if speed, ok := state["port-speed"].(string); ok && speed != "SPEED_UNKNOWN" {
				ethConfig.PortSpeed = stripNamespace(speed)
				hasConfig = true
			}
		}

		if duplex, ok := state["duplex-mode"].(string); ok {
			ethConfig.DuplexMode = duplex
			hasConfig = true
		}

		if ethConfig.MacAddress == "" {
			if mac, ok := state["hw-mac-address"].(string); ok {
				ethConfig.MacAddress = mac
				hasConfig = true
			}
		}
	}

	if hasConfig {
		iface.Ethernet = ethConfig
	}
}

func (e *InterfacesExporter) parseAggregation(iface *model.Interface, agg map[string]interface{}) {
	lagConfig := &model.LAGConfig{}
	hasConfig := false

	if config, ok := agg["config"].(map[string]interface{}); ok {
		if lagType, ok := config["lag-type"].(string); ok {
			lagConfig.LACPMode = stripNamespace(lagType)
			hasConfig = true
		}
	}

	// Check for aggregate-id (member of LAG)
	if config, ok := agg["config"].(map[string]interface{}); ok {
		if aggId, ok := config["aggregate-id"].(string); ok {
			lagConfig.AggregateID = aggId
			hasConfig = true
		}
	}

	if hasConfig {
		iface.LAG = lagConfig
	}
}

func (e *InterfacesExporter) parseSubinterfaces(iface *model.Interface, subints map[string]interface{}) {
	var subintList []interface{}

	if subs, ok := subints["subinterface"].([]interface{}); ok {
		subintList = subs
	}

	for _, sub := range subintList {
		subData, ok := sub.(map[string]interface{})
		if !ok {
			continue
		}

		// Parse IPv4
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

func (e *InterfacesExporter) parseVXLAN(iface *model.Interface, vxlan map[string]interface{}) {
	vxlanConfig := &model.VXLANConfig{}
	hasConfig := false

	// Parse config block
	if config, ok := vxlan["config"].(map[string]interface{}); ok {
		if srcIface, ok := config["source-interface"].(string); ok {
			vxlanConfig.SourceInterface = srcIface
			hasConfig = true
		}
		if port, ok := config["udp-port"].(float64); ok && port > 0 {
			vxlanConfig.UDPPort = int(port)
			hasConfig = true
		}
	}

	// Parse VNI mappings from vlan-to-vnis
	if vlanToVnis, ok := vxlan["vlan-to-vnis"].(map[string]interface{}); ok {
		if vniList, ok := vlanToVnis["vlan-to-vni"].([]interface{}); ok {
			for _, item := range vniList {
				if mapping, ok := item.(map[string]interface{}); ok {
					vni := 0
					vlan := 0
					
					// Get VNI from config or state
					if config, ok := mapping["config"].(map[string]interface{}); ok {
						if v, ok := config["vni"].(float64); ok {
							vni = int(v)
						}
					}
					if state, ok := mapping["state"].(map[string]interface{}); ok {
						if v, ok := state["vni"].(float64); ok {
							vni = int(v)
						}
					}
					
					// Get VLAN from vlan key
					if v, ok := mapping["vlan"].(float64); ok {
						vlan = int(v)
					}
					
					if vni > 0 {
						vxlanConfig.VNIMappings = append(vxlanConfig.VNIMappings, model.VNIMapping{
							VNI:  vni,
							VLAN: vlan,
						})
						hasConfig = true
					}
				}
			}
		}
	}

	if hasConfig {
		iface.VXLAN = vxlanConfig
	}
}

func (e *InterfacesExporter) Apply(m *model.DeviceModel) {
	if len(e.interfaces) > 0 {
		m.Interfaces = e.interfaces
	}
}
