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

	// Get AAA config (users)
	aaaData, err := client.GetJSON(ctx, "/system/aaa")
	if err == nil && aaaData != nil {
		e.parseAAA(aaaData)
	}

	// Get logging config
	loggingData, err := client.GetJSON(ctx, "/system/logging")
	if err == nil && loggingData != nil {
		e.parseLogging(loggingData)
	}

	return nil
}

func (e *SystemExporter) parseConfig(data map[string]interface{}) {
	// Handle nested config container (Arista returns this structure)
	config := data
	if cfg, ok := data["openconfig-system:config"].(map[string]interface{}); ok {
		config = cfg
	} else if cfg, ok := data["config"].(map[string]interface{}); ok {
		config = cfg
	}

	if hostname, ok := config["openconfig-system:hostname"].(string); ok {
		e.system.Hostname = hostname
		e.metadata.Hostname = hostname
	} else if hostname, ok := config["hostname"].(string); ok {
		e.system.Hostname = hostname
		e.metadata.Hostname = hostname
	}

	if domain, ok := config["openconfig-system:domain-name"].(string); ok {
		e.system.DomainName = domain
	} else if domain, ok := config["domain-name"].(string); ok {
		e.system.DomainName = domain
	}
}

func (e *SystemExporter) parseState(data map[string]interface{}) {
	// Handle nested state container
	state := data
	if st, ok := data["openconfig-system:state"].(map[string]interface{}); ok {
		state = st
	} else if st, ok := data["state"].(map[string]interface{}); ok {
		state = st
	}

	if hostname, ok := state["openconfig-system:hostname"].(string); ok {
		if e.metadata.Hostname == "" {
			e.metadata.Hostname = hostname
		}
	} else if hostname, ok := state["hostname"].(string); ok {
		if e.metadata.Hostname == "" {
			e.metadata.Hostname = hostname
		}
	}

	if version, ok := state["openconfig-system:software-version"].(string); ok {
		e.metadata.Version = version
	} else if version, ok := state["software-version"].(string); ok {
		e.metadata.Version = version
	}

	if model, ok := state["openconfig-system:hardware-model"].(string); ok {
		e.metadata.Model = model
	} else if model, ok := state["hardware-model"].(string); ok {
		e.metadata.Model = model
	}

	if serial, ok := state["openconfig-system:serial-number"].(string); ok {
		e.metadata.Serial = serial
	} else if serial, ok := state["serial-number"].(string); ok {
		e.metadata.Serial = serial
	}
}

func (e *SystemExporter) parseNTP(data map[string]interface{}) {
	ntp := &model.NTP{}

	config := data
	if cfg, ok := data["openconfig-system:config"].(map[string]interface{}); ok {
		config = cfg
	} else if cfg, ok := data["config"].(map[string]interface{}); ok {
		config = cfg
	}

	if enabled, ok := config["enabled"].(bool); ok {
		ntp.Enabled = enabled
	}

	// Parse servers
	if servers, ok := data["openconfig-system:servers"].(map[string]interface{}); ok {
		e.parseNTPServers(ntp, servers)
	} else if servers, ok := data["servers"].(map[string]interface{}); ok {
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

	if servers, ok := data["openconfig-system:servers"].(map[string]interface{}); ok {
		e.parseDNSServers(dns, servers)
	} else if servers, ok := data["servers"].(map[string]interface{}); ok {
		e.parseDNSServers(dns, servers)
	}

	config := data
	if cfg, ok := data["openconfig-system:config"].(map[string]interface{}); ok {
		config = cfg
	} else if cfg, ok := data["config"].(map[string]interface{}); ok {
		config = cfg
	}

	if search, ok := config["search"].([]interface{}); ok {
		for _, s := range search {
			if str, ok := s.(string); ok {
				dns.Search = append(dns.Search, str)
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

		if addr, ok := srvData["address"].(string); ok {
			dns.Servers = append(dns.Servers, addr)
		} else if config, ok := srvData["config"].(map[string]interface{}); ok {
			if addr, ok := config["address"].(string); ok {
				dns.Servers = append(dns.Servers, addr)
			}
		}
	}
}

func (e *SystemExporter) parseAAA(data map[string]interface{}) {
	aaa := &model.AAA{}

	// Try to get authentication/users
	var auth map[string]interface{}
	if a, ok := data["openconfig-system:authentication"].(map[string]interface{}); ok {
		auth = a
	} else if a, ok := data["authentication"].(map[string]interface{}); ok {
		auth = a
	}

	if auth != nil {
		if users, ok := auth["users"].(map[string]interface{}); ok {
			e.parseUsers(aaa, users)
		}
	}

	if len(aaa.Users) > 0 {
		e.system.AAA = aaa
	}
}

func (e *SystemExporter) parseUsers(aaa *model.AAA, data map[string]interface{}) {
	var userList []interface{}

	if users, ok := data["user"].([]interface{}); ok {
		userList = users
	}

	for _, u := range userList {
		userData, ok := u.(map[string]interface{})
		if !ok {
			continue
		}

		user := model.User{}

		if username, ok := userData["username"].(string); ok {
			user.Username = username
		}

		if config, ok := userData["config"].(map[string]interface{}); ok {
			if role, ok := config["role"].(string); ok {
				user.Role = stripNamespace(role)
			}
		}

		// Check for SSH keys
		if sshServer, ok := userData["ssh-server"].(map[string]interface{}); ok {
			if authorizedKeys, ok := sshServer["authorized-keys"].(map[string]interface{}); ok {
				if keyList, ok := authorizedKeys["authorized-key"].([]interface{}); ok {
					for _, k := range keyList {
						if keyData, ok := k.(map[string]interface{}); ok {
							if state, ok := keyData["state"].(map[string]interface{}); ok {
								if keyType, ok := state["key-type"].(string); ok {
									if keyValue, ok := state["key-data"].(string); ok {
										user.SSHKey = keyType + " " + keyValue
										break // Just get first key
									}
								}
							}
						}
					}
				}
			}
		}

		if user.Username != "" {
			aaa.Users = append(aaa.Users, user)
		}
	}
}

func (e *SystemExporter) parseLogging(data map[string]interface{}) {
	logging := &model.Logging{}

	// Try to get remote servers
	var remoteServers map[string]interface{}
	if rs, ok := data["openconfig-system:remote-servers"].(map[string]interface{}); ok {
		remoteServers = rs
	} else if rs, ok := data["remote-servers"].(map[string]interface{}); ok {
		remoteServers = rs
	}

	if remoteServers != nil {
		var serverList []interface{}
		if srvs, ok := remoteServers["remote-server"].([]interface{}); ok {
			serverList = srvs
		}

		for _, srv := range serverList {
			srvData, ok := srv.(map[string]interface{})
			if !ok {
				continue
			}

			logServer := model.LogServer{}

			if host, ok := srvData["host"].(string); ok {
				logServer.Address = host
			}

			if config, ok := srvData["config"].(map[string]interface{}); ok {
				if host, ok := config["host"].(string); ok {
					logServer.Address = host
				}
				if port, ok := config["remote-port"].(float64); ok {
					logServer.Port = int(port)
				}
				if protocol, ok := config["transport"].(string); ok {
					logServer.Protocol = stripNamespace(protocol)
				}
			}

			// Check selectors for facility
			if selectors, ok := srvData["selectors"].(map[string]interface{}); ok {
				if selectorList, ok := selectors["selector"].([]interface{}); ok {
					for _, sel := range selectorList {
						if selData, ok := sel.(map[string]interface{}); ok {
							if config, ok := selData["config"].(map[string]interface{}); ok {
								if facility, ok := config["facility"].(string); ok {
									logServer.Facility = stripNamespace(facility)
									break
								}
							}
						}
					}
				}
			}

			if logServer.Address != "" {
				logging.Servers = append(logging.Servers, logServer)
			}
		}
	}

	if len(logging.Servers) > 0 {
		e.system.Logging = logging
	}
}

func (e *SystemExporter) Apply(m *model.DeviceModel) {
	if e.system.Hostname != "" || e.system.DomainName != "" || e.system.NTP != nil || e.system.DNS != nil || e.system.AAA != nil || e.system.Logging != nil {
		m.System = e.system
	}

	if e.metadata.Hostname != "" || e.metadata.Version != "" {
		m.Metadata = *e.metadata
	}
}
