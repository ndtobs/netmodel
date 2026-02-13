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

## Deduplication — The Killer Feature

When exporting multiple devices with `--structure ansible --dedup`, netmodel analyzes all configs and extracts common configuration automatically:

**Before (without --dedup):** Everything duplicated in each host

```
host_vars/
├── leaf1/
│   ├── bgp.yaml         # peer_groups, neighbors, global
│   ├── system.yaml      # hostname, NTP, DNS, AAA
│   └── routing_policy.yaml
├── leaf2/
│   ├── bgp.yaml         # same peer_groups duplicated!
│   ├── system.yaml      # same NTP/DNS/AAA duplicated!
│   └── routing_policy.yaml
└── ...
```

**After (with --dedup):** Common config extracted, DRY principle applied

```
group_vars/
├── all.yaml             # NTP, DNS, AAA (common to ALL devices)
├── spine.yaml           # Spine-only peer groups, policies
└── leaf.yaml            # Leaf-only peer groups, policies
host_vars/
├── leaf1/
│   ├── bgp.yaml         # Just: router_id, neighbors (device-specific)
│   └── interfaces.yaml
├── leaf2/
│   └── ...              # Much smaller!
```

### Real Example

**group_vars/leaf.yaml** — extracted automatically (identical across all leaves):
```yaml
bgp:
  peer_groups:
    SPINE:
      peer_as: 65000
      afi_safi:
        - name: IPV4_UNICAST
    SPINE-EVPN:
      peer_as: 65000
      update_source: Loopback0
      ebgp_multihop: 3
      afi_safi:
        - name: L2VPN_EVPN
routing_policy:
  defined_sets:
    prefix_sets:
      - name: LOOPBACKS
        prefixes:
          - prefix: 10.255.0.0/16
            mask_range: 16..32
system:
  aaa:
    users:
      - username: admin
        role: network-admin
```

**host_vars/leaf1/bgp.yaml** — only device-specific config remains:
```yaml
bgp:
  global:
    as: 65001
    router_id: 10.255.1.1
  neighbors:
    10.0.0.0:
      peer_group: SPINE
    10.0.0.4:
      peer_group: SPINE
    10.255.0.1:
      peer_group: SPINE-EVPN
    10.255.0.2:
      peer_group: SPINE-EVPN
```

This follows Ansible best practices — change NTP servers once in `group_vars/all.yaml`, not in 50 host files.

## Try It

A test lab is included (requires [containerlab](https://containerlab.dev) and cEOS image):

```bash
# Deploy lab
cd examples/lab
sudo clab deploy -t topology.yaml

# Wait ~60s for boot, then export
cd ../..
netmodel export @all -i examples/lab/inventory.yaml -o /tmp/no-dedup --structure ansible
netmodel export @all -i examples/lab/inventory.yaml -o /tmp/with-dedup --structure ansible --dedup

# Compare
tree /tmp/no-dedup /tmp/with-dedup
cat /tmp/with-dedup/group_vars/leaf.yaml

# Cleanup
cd examples/lab && sudo clab destroy -t topology.yaml
```

## Features

| Exporter | Description |
|----------|-------------|
| `interfaces` | IPs, descriptions, MTU, ethernet, LAG |
| `bgp` | Global config, peer groups, neighbors, AFI/SAFI |
| `ospf` | Areas, interfaces, timers |
| `system` | Hostname, NTP, DNS, AAA/users, syslog |
| `routing_policy` | Prefix-sets, community-sets, policies |
| `evpn` | VTEP, VLAN-VNI, VRF-VNI mappings |

## Documentation

Full documentation: **[rob0t.tools/docs/netmodel](https://rob0t.tools/docs/netmodel/)**

- [Exporters](https://rob0t.tools/docs/netmodel/exporters/) — All exporters and output format
- [Inventory](https://rob0t.tools/docs/netmodel/inventory/) — Organize devices into groups
- [Ansible](https://rob0t.tools/docs/netmodel/ansible/) — Ansible integration

## Related

- **[netsert](https://github.com/ndtobs/netsert)** — Validate network state against assertions

## License

MIT
