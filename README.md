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

## Documentation

Full documentation: **[rob0t.tools/docs/netmodel](https://rob0t.tools/docs/netmodel/)**

- [Exporters](https://rob0t.tools/docs/netmodel/exporters/) — All exporters and output format
- [Inventory](https://rob0t.tools/docs/netmodel/inventory/) — Organize devices into groups
- [Ansible](https://rob0t.tools/docs/netmodel/ansible/) — Ansible integration

## Related

- **[netsert](https://github.com/ndtobs/netsert)** — Validate network state against assertions

## License

MIT
