# netmodel

Network data model generator — extract OpenConfig-based YAML from live networks via gNMI.

```
Live Network → netmodel export → YAML Data Model → Ansible / IaC Tools
```

## Install

```bash
go install github.com/ndtobs/netmodel/cmd/netmodel@latest
```

## Quick Start

```bash
# Export from a single device
netmodel export 10.0.0.1:6030 -u admin -P password -k

# Export specific features
netmodel export 10.0.0.1:6030 --features interfaces,bgp -o spine1/

# Export inventory group with Ansible structure
netmodel export @all -i inventory.yaml -o ./network-model/ --structure ansible

# Export with deduplication (extracts common config to group_vars)
netmodel export @all -i inventory.yaml -o ./network-model/ --structure ansible --dedup
```

## Example Output

```yaml
# interfaces.yaml
interfaces:
  Ethernet1:
    description: "Uplink to leaf1"
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
```

## Features

| Exporter | Description |
|----------|-------------|
| `interfaces` | IPs, descriptions, MTU, ethernet, LAG |
| `bgp` | Global config, peer groups, neighbors, AFI/SAFI |
| `ospf` | Areas, interfaces, timers |
| `system` | Hostname, NTP, DNS, AAA/users, syslog |
| `routing_policy` | Prefix-sets, community-sets, policies |

## Deduplication

When exporting multiple devices with `--structure ansible --dedup`, netmodel analyzes all exported configs and extracts common configuration:

- **`group_vars/all.yaml`** — Config identical across ALL devices (NTP servers, DNS, common peer groups)
- **`group_vars/<group>.yaml`** — Config identical within inventory groups (spine-specific, leaf-specific)
- **`host_vars/<device>/`** — Device-specific config only (router-id, neighbors, interfaces)

```
network-model/
├── group_vars/
│   ├── all.yaml        # NTP, DNS, AAA (common to all)
│   ├── spine.yaml      # Spine peer groups
│   └── leaf.yaml       # Leaf peer groups
└── host_vars/
    ├── spine1/
    │   ├── bgp.yaml    # router_id, neighbors
    │   └── interfaces.yaml
    └── leaf1/
        └── ...
```

This follows Ansible best practices — common config in one place, device-specific overrides where needed.

## Documentation

Full documentation: **[rob0t.tools/docs/netmodel](https://rob0t.tools/docs/netmodel/)**

- [Exporters](https://rob0t.tools/docs/netmodel/exporters/) — All exporters and output format
- [Inventory](https://rob0t.tools/docs/netmodel/inventory/) — Organize devices into groups
- [Ansible](https://rob0t.tools/docs/netmodel/ansible/) — Ansible integration

## Related

- **[netsert](https://github.com/ndtobs/netsert)** — Validate network state against assertions

## License

MIT
