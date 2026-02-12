package exporter

import (
	"context"
	"strconv"
	"strings"

	"github.com/ndtobs/netmodel/internal/gnmi"
	"github.com/ndtobs/netmodel/internal/model"
)

// EVPNExporter exports EVPN/VXLAN configuration
type EVPNExporter struct {
	evpn *model.EVPN
}

func (e *EVPNExporter) Name() string {
	return "evpn"
}

func (e *EVPNExporter) Export(ctx context.Context, client *gnmi.Client) error {
	e.evpn = &model.EVPN{
		VLANVNIs: make(map[string]*model.VLANVNI),
		VRFVNIs:  make(map[string]*model.VRFVNI),
	}

	// Get VXLAN interface config (Arista-specific path)
	e.getVxlanInterface(ctx, client)

	// Get BGP EVPN VLAN configs (RD/RT from BGP section)
	e.getBGPEVPNConfig(ctx, client)

	return nil
}

func (e *EVPNExporter) getVxlanInterface(ctx context.Context, client *gnmi.Client) {
	// Query Vxlan1 interface directly
	vxlanData, err := client.GetJSON(ctx, "/interfaces/interface[name=Vxlan1]")
	if err != nil {
		return
	}

	// Parse Arista VXLAN extensions
	if aristaVxlan, ok := vxlanData["arista-exp-eos-vxlan:arista-vxlan"].(map[string]interface{}); ok {
		// Get source interface and UDP port from config
		if config, ok := aristaVxlan["config"].(map[string]interface{}); ok {
			if src, ok := config["src-ip-intf"].(string); ok && src != "" {
				e.evpn.VTEPSource = src
			}
			if port, ok := config["udp-port"].(float64); ok {
				e.evpn.UDPPort = int(port)
			}
		}

		// Parse VLAN to VNI mappings
		if vlanVniMap, ok := aristaVxlan["vlan-to-vnis"].(map[string]interface{}); ok {
			if vlanVnis, ok := vlanVniMap["vlan-to-vni"].([]interface{}); ok {
				for _, vv := range vlanVnis {
					vvMap, ok := vv.(map[string]interface{})
					if !ok {
						continue
					}
					
					var vlan, vni int
					
					// Get VLAN from top level or config
					if v, ok := vvMap["vlan"].(float64); ok {
						vlan = int(v)
					}
					
					// Get VNI from config
					if config, ok := vvMap["config"].(map[string]interface{}); ok {
						if v, ok := config["vni"].(float64); ok {
							vni = int(v)
						}
					}
					
					if vlan > 0 && vni > 0 {
						key := strconv.Itoa(vlan)
						e.evpn.VLANVNIs[key] = &model.VLANVNI{
							VLAN: vlan,
							VNI:  vni,
						}
					}
				}
			}
		}

		// Parse VRF to L3VNI mappings
		if vrfVniMap, ok := aristaVxlan["vrf-to-vnis"].(map[string]interface{}); ok {
			if vrfVnis, ok := vrfVniMap["vrf-to-vni"].([]interface{}); ok {
				for _, vv := range vrfVnis {
					vvMap, ok := vv.(map[string]interface{})
					if !ok {
						continue
					}
					
					var vrf string
					var vni int
					
					// Get VRF from top level or config
					if v, ok := vvMap["vrf"].(string); ok {
						vrf = v
					}
					
					// Get VNI from config
					if config, ok := vvMap["config"].(map[string]interface{}); ok {
						if v, ok := config["vni"].(float64); ok {
							vni = int(v)
						}
					}
					
					if vrf != "" && vni > 0 {
						e.evpn.VRFVNIs[vrf] = &model.VRFVNI{
							VRF: vrf,
							VNI: vni,
						}
					}
				}
			}
		}
	}
}

func (e *EVPNExporter) getBGPEVPNConfig(ctx context.Context, client *gnmi.Client) {
	// BGP EVPN per-VLAN config (RD/RT) is typically under BGP
	// On Arista this may be under arista-bgp-augments:vlans
	// For now, we get the basic VNI mappings from VXLAN interface
	// RD/RT config would need additional path investigation
	
	// Try to query network-instances for VRF EVPN settings
	e.getVRFEVPNConfig(ctx, client)
}

func (e *EVPNExporter) getVRFEVPNConfig(ctx context.Context, client *gnmi.Client) {
	// Get VRF-specific EVPN config (RD/RT)
	// Query each VRF we found in VXLAN config
	for vrfName, vrfVNI := range e.evpn.VRFVNIs {
		vrfData, err := client.GetJSON(ctx, "/network-instances/network-instance[name="+vrfName+"]")
		if err != nil {
			continue
		}

		// Look for config with RD
		if config, ok := vrfData["openconfig-network-instance:config"].(map[string]interface{}); ok {
			if rd, ok := config["route-distinguisher"].(string); ok {
				vrfVNI.RD = rd
			}
		}

		// Look for inter-instance policies with route targets
		if policies, ok := vrfData["openconfig-network-instance:inter-instance-policies"].(map[string]interface{}); ok {
			if applyPolicy, ok := policies["apply-policy"].(map[string]interface{}); ok {
				if config, ok := applyPolicy["config"].(map[string]interface{}); ok {
					if importRTs, ok := config["import-policy"].([]interface{}); ok {
						for _, rt := range importRTs {
							if rtStr, ok := rt.(string); ok {
								vrfVNI.RouteTargetImport = append(vrfVNI.RouteTargetImport, rtStr)
							}
						}
					}
					if exportRTs, ok := config["export-policy"].([]interface{}); ok {
						for _, rt := range exportRTs {
							if rtStr, ok := rt.(string); ok {
								vrfVNI.RouteTargetExport = append(vrfVNI.RouteTargetExport, rtStr)
							}
						}
					}
				}
			}
		}
	}
}

func (e *EVPNExporter) Apply(m *model.DeviceModel) {
	// Only apply if we have meaningful EVPN config
	if e.evpn.VTEPSource != "" || len(e.evpn.VLANVNIs) > 0 || len(e.evpn.VRFVNIs) > 0 {
		m.EVPN = e.evpn
	}
}

// Helper to strip namespace prefix
func stripNS(s string) string {
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}
