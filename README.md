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
netmodel export @spine -i inventory.yaml -o ./network-model/
```

## Output Format

```yaml
metadata:
  hostname: spine1
  version: 4.28.0F
  exported_at: 2026-02-12T13:00:00Z
  netmodel_version: 0.1.0

interfaces:
  Ethernet1:
    description: "Uplink to leaf1"
    enabled: true
    mtu: 9214
    ipv4:
      addresses:
        - ip: 10.0.0.0
          prefix_length: 31

  Loopback0:
    description: "Router ID"
    enabled: true
    ipv4:
      addresses:
        - ip: 10.255.0.1
          prefix_length: 32

bgp:
  global:
    as: 65001
    router_id: 10.255.0.1
  neighbors:
    10.0.0.1:
      description: "leaf1"
      enabled: true
      peer_as: 65101

system:
  hostname: spine1
  domain_name: lab.local
  ntp:
    enabled: true
    servers:
      - address: 10.0.0.250
        prefer: true
```

## Features

| Feature | Description |
|---------|-------------|
| `interfaces` | Interface configuration (description, enabled, MTU, IP addresses) |
| `bgp` | BGP global config, peer groups, and neighbors |
| `system` | Hostname, domain, NTP, DNS |

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
    - 10.0.0.1:6030
    - 10.0.0.2:6030
  leaf:
    - 10.0.0.11:6030
    - 10.0.0.12:6030
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

## Output Options

```bash
# Single file
netmodel export 10.0.0.1:6030 -o spine1.yaml

# Directory (one file per device)
netmodel export @all -i inventory.yaml -o ./network-model/

# Split mode (per-feature files)
netmodel export @all -i inventory.yaml -o ./network-model/ --split
```

Split mode creates:

```
network-model/
├── spine1/
│   ├── metadata.yaml
│   ├── interfaces.yaml
│   ├── bgp.yaml
│   └── system.yaml
└── spine2/
    └── ...
```

## Ansible Integration

Use the exported YAML as Ansible vars:

```yaml
# playbook.yaml
- hosts: network
  vars_files:
    - "network-model/{{ inventory_hostname }}.yaml"
  tasks:
    - name: Configure interfaces
      arista.eos.eos_interfaces:
        config: "{{ interfaces | dict2items | map(attribute='value') | list }}"
```

## Related Tools

- **[netsert](https://github.com/ndtobs/netsert)** — Validate network state against assertions (pairs with netmodel)
- **netmodel** generates the data model, **netsert** validates it matches reality

## License

MIT
