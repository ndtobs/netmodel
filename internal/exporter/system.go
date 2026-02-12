package exporter

import (
	"context"

	"github.com/ndtobs/netmodel/internal/gnmi"
	"github.com/ndtobs/netmodel/internal/model"
)

// SystemExporter exports system configuration
type SystemExporter struct {
	system   *model.System
	metadata *model.Metadata
}

func (e *SystemExporter) Name() string {
	return "system"
}

func (e *SystemExporter) Export(ctx context.Context, client *gnmi.Client) error {
	e.system = &model.System{}
	e.metadata = &model.Metadata{}

	// Get system config (hostname, domain-name)
	configData, err := client.GetJSON(ctx, "/system/config")
	if err == nil && configData != nil {
		e.parseConfig(configData)
	}

	// Get system state (for metadata like version, model)
	stateData, err := client.GetJSON(ctx, "/system/state")
	if err == nil && stateData != nil {
		e.parseState(stateData)
	}

	// Get NTP config
	ntpData, err := client.GetJSON(ctx, "/system/ntp")
	if err == nil && ntpData != nil {
		e.parseNTP(ntpData)
	}

	// Get DNS config
	dnsData, err := client.GetJSON(ctx, "/system/dns")
	if err == nil && dnsData != nil {
		e.parseDNS(dnsData)
	}

	return nil
}

func (e *SystemExporter) parseConfig(data map[string]interface{}) {
	config := data
	if cfg, ok := data["openconfig-system:config"].(map[string]interface{}); ok {
		config = cfg
	}

	if hostname, ok := config["hostname"].(string); ok {
		e.system.Hostname = hostname
		e.metadata.Hostname = hostname
	}

	if domain, ok := config["domain-name"].(string); ok {
		e.system.DomainName = domain
	}
}

func (e *SystemExporter) parseState(data map[string]interface{}) {
	state := data
	if st, ok := data["openconfig-system:state"].(map[string]interface{}); ok {
		state = st
	}

	if hostname, ok := state["hostname"].(string); ok {
		if e.metadata.Hostname == "" {
			e.metadata.Hostname = hostname
		}
		if e.system.Hostname == "" {
			e.system.Hostname = hostname
		}
	}

	// Software version
	if version, ok := state["software-version"].(string); ok {
		e.metadata.Version = version
	}

	// Hardware info might be in different places
	if model, ok := state["hardware-model"].(string); ok {
		e.metadata.Model = model
	}

	if serial, ok := state["serial-number"].(string); ok {
		e.metadata.Serial = serial
	}
}

func (e *SystemExporter) parseNTP(data map[string]interface{}) {
	ntp := &model.NTP{}

	// Check for config
	config := data
	if cfg, ok := data["config"].(map[string]interface{}); ok {
		config = cfg
	}

	if enabled, ok := config["enabled"].(bool); ok {
		ntp.Enabled = enabled
	}

	// Parse servers
	if servers, ok := data["servers"].(map[string]interface{}); ok {
		e.parseNTPServers(ntp, servers)
	} else if servers, ok := data["openconfig-system:servers"].(map[string]interface{}); ok {
		e.parseNTPServers(ntp, servers)
	}

	if ntp.Enabled || len(ntp.Servers) > 0 {
		e.system.NTP = ntp
	}
}

func (e *SystemExporter) parseNTPServers(ntp *model.NTP, data map[string]interface{}) {
	var serverList []interface{}

	if srvs, ok := data["server"].([]interface{}); ok {
		serverList = srvs
	}

	for _, srv := range serverList {
		srvData, ok := srv.(map[string]interface{})
		if !ok {
			continue
		}

		server := model.NTPServer{}

		// Address might be at top level or in config
		if addr, ok := srvData["address"].(string); ok {
			server.Address = addr
		}

		config, ok := srvData["config"].(map[string]interface{})
		if ok {
			if addr, ok := config["address"].(string); ok {
				server.Address = addr
			}
			if prefer, ok := config["prefer"].(bool); ok {
				server.Prefer = prefer
			}
		}

		if server.Address != "" {
			ntp.Servers = append(ntp.Servers, server)
		}
	}
}

func (e *SystemExporter) parseDNS(data map[string]interface{}) {
	dns := &model.DNS{}

	// Parse servers
	if servers, ok := data["servers"].(map[string]interface{}); ok {
		e.parseDNSServers(dns, servers)
	} else if servers, ok := data["openconfig-system:servers"].(map[string]interface{}); ok {
		e.parseDNSServers(dns, servers)
	}

	// Parse search domains
	if config, ok := data["config"].(map[string]interface{}); ok {
		if search, ok := config["search"].([]interface{}); ok {
			for _, s := range search {
				if str, ok := s.(string); ok {
					dns.Search = append(dns.Search, str)
				}
			}
		}
	}

	if len(dns.Servers) > 0 || len(dns.Search) > 0 {
		e.system.DNS = dns
	}
}

func (e *SystemExporter) parseDNSServers(dns *model.DNS, data map[string]interface{}) {
	var serverList []interface{}

	if srvs, ok := data["server"].([]interface{}); ok {
		serverList = srvs
	}

	for _, srv := range serverList {
		srvData, ok := srv.(map[string]interface{})
		if !ok {
			continue
		}

		// Address might be at top level or in config
		if addr, ok := srvData["address"].(string); ok {
			dns.Servers = append(dns.Servers, addr)
		} else if config, ok := srvData["config"].(map[string]interface{}); ok {
			if addr, ok := config["address"].(string); ok {
				dns.Servers = append(dns.Servers, addr)
			}
		}
	}
}

func (e *SystemExporter) Apply(m *model.DeviceModel) {
	// Apply system config
	if e.system.Hostname != "" || e.system.DomainName != "" || e.system.NTP != nil || e.system.DNS != nil {
		m.System = e.system
	}

	// Apply metadata
	if e.metadata.Hostname != "" || e.metadata.Version != "" {
		m.Metadata = *e.metadata
	}
}
