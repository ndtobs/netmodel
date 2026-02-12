# netmodel

Network data model generator — extract OpenConfig-based YAML from live networks via gNMI.

## Overview

`netmodel` connects to network devices via gNMI and exports their configuration to a clean, vendor-agnostic YAML data model. This YAML becomes your source of truth for Infrastructure as Code (IaC) workflows with tools like Ansible.

```
Live Network → netmodel export → YAML Data Model → Ansible / IaC Tools
```

## Installation

```bash
go install github.com/ndtobs/netmodel/cmd/netmodel@latest
```

Or build from source:

```bash
git clone https://github.com/ndtobs/netmodel
cd netmodel
go build -o netmodel ./cmd/netmodel
```

## Quick Start

Export configuration from a single device:

```bash
netmodel export 10.0.0.1:6030 -u admin -P password -k
```

Export specific features:

```bash
netmodel export 10.0.0.1:6030 --features interfaces,bgp -o spine1.yaml
```

Export from all devices in a group:

```bash
netmodel export @all -i inventory.yaml -o ./network-model/
```

Export with Ansible structure:

```bash
netmodel export @all -i inventory.yaml -o ./network-model/ --structure ansible
```

## Output Format

### Default (Flat Structure)

```
network-model/
├── spine1/
│   ├── metadata.yaml
│   ├── interfaces.yaml
│   ├── bgp.yaml
│   ├── ospf.yaml
│   ├── system.yaml
│   └── routing_policy.yaml
└── leaf1/
    └── ...
```

### Ansible Structure (`--structure ansible`)

```
network-model/
├── group_vars/
│   └── all.yaml          # common variables (v0.2+)
└── host_vars/
    ├── spine1/
    │   ├── metadata.yaml
    │   ├── interfaces.yaml
    │   ├── bgp.yaml
    │   ├── ospf.yaml
    │   ├── system.yaml
    │   └── routing_policy.yaml
    └── leaf1/
        └── ...
```

### Example Output

```yaml
# metadata.yaml
hostname: spine1
version: 4.28.0F
exported_at: 2026-02-12T13:00:00Z
netmodel_version: 0.1.0

# interfaces.yaml
interfaces:
  Ethernet1:
    description: "Uplink to leaf1"
    type: ethernetCsmacd
    ipv4:
      addresses:
        - ip: 10.0.0.0
          prefix_length: 31

  Loopback0:
    type: softwareLoopback
    ipv4:
      addresses:
        - ip: 10.255.0.1
          prefix_length: 32

# bgp.yaml
bgp:
  global:
    as: 65001
    router_id: 10.255.0.1
  neighbors:
    10.0.0.1:
      peer_as: 65101

# system.yaml
system:
  hostname: spine1
  domain_name: lab.local
```

## Features

| Feature | Description |
|---------|-------------|
| `interfaces` | Interface configuration (description, enabled, MTU, IP addresses, ethernet, LAG) |
| `bgp` | BGP global config, peer groups, neighbors (AFI/SAFI, timers, policies) |
| `ospf` | OSPF areas, interfaces, network types, timers |
| `system` | Hostname, domain, NTP, DNS, AAA/users, logging/syslog |
| `routing_policy` | Prefix-sets, community-sets, as-path-sets, policy-definitions |

List available features:

```bash
netmodel features
```

## Inventory

Use an inventory file to organize devices into groups:

```yaml
# inventory.yaml
groups:
  spine:
    - spine1:6030
    - spine2:6030
  leaf:
    - leaf1:6030
    - leaf2:6030
  all:
    - "@spine"
    - "@leaf"

defaults:
  username: admin
  password: admin
  insecure: true
```

Then export by group:

```bash
netmodel export @spine -i inventory.yaml -o ./network-model/
```

## CLI Reference

```
netmodel export <target> [flags]

Flags:
  -u, --username string    gNMI username
  -P, --password string    gNMI password
  -k, --insecure           skip TLS verification
  -f, --features strings   features to export (default: all)
  -o, --output string      output path (file or directory)
  -i, --inventory string   inventory file for @group targets
  -s, --structure string   output structure: flat, ansible (default: flat)
      --no-split           single file per device (default: split per-feature)
  -t, --timeout duration   gNMI timeout (default: 30s)
```

## Ansible Integration

With `--structure ansible`, output is ready for Ansible:

```yaml
# playbook.yaml
- hosts: network
  tasks:
    - name: Configure interfaces
      arista.eos.eos_l3_interfaces:
        config: "{{ interfaces | dict2items | map(attribute='value') | list }}"
```

Variables from `host_vars/<hostname>/` are automatically loaded by Ansible.

## Roadmap

- [x] v0.1: Core export functionality (interfaces, bgp, ospf, system, routing_policy)
- [ ] v0.2: Config deduplication (extract common config to group_vars)
- [ ] v0.3: Diff command (compare live vs model)
- [ ] v0.4: Additional features (VLANs, LLDP)

## Related Tools

- **[netsert](https://github.com/ndtobs/netsert)** — Validate network state against assertions
- **netmodel** generates the data model, **netsert** validates it matches reality

## License

MIT
