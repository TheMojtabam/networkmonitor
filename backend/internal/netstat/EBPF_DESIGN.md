# eBPF Per-Port Byte Accounting Design

## Overview

This document describes the architecture for implementing per-port byte tracking using eBPF.

## Approach

### Hook Points

1. **TC (Traffic Control) Egress/Ingress**
   - Attach to network interface
   - See all packets entering/leaving
   - Supports both IPv4 and IPv6

2. **XDP (eXpress Data Path)**
   - Earlier in the pipeline (fastest)
   - Only sees RX path
   - Need separate TX hook

3. **Cgroup-based Socket Hooks**
   - Attach to socket creation
   - Requires cgroup v2

**Recommendation:** TC egress/ingress for comprehensive coverage.

## BPF Map Structure

### Map 1: Port Statistics
```c
struct port_key {
    __u16 port;
    __u8 protocol; // IPPROTO_TCP (6) or IPPROTO_UDP (17)
    __u8 direction; // 0=RX, 1=TX
};

struct port_stats {
    __u64 bytes;
    __u64 packets;
};

// BPF map
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct port_key);
    __type(value, struct port_stats);
    __uint(max_entries, 65536); // 64K ports * protocols * directions
} port_stats_map SEC(".maps");
```

## BPF Program Logic

### Ingress (RX)
```c
SEC("tc/ingress")
int tc_ingress(struct __sk_buff *skb) {
    void *data_end = (void *)(long)skb->data_end;
    void *data = (void *)(long)skb->data;
    
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return TC_ACT_OK;
    
    if (eth->h_proto == htons(ETH_P_IP)) {
        struct iphdr *ip = data + sizeof(*eth);
        if ((void *)(ip + 1) > data_end)
            return TC_ACT_OK;
        
        __u16 dport = 0;
        __u8 proto = ip->protocol;
        
        if (proto == IPPROTO_TCP) {
            struct tcphdr *tcp = (void *)ip + (ip->ihl * 4);
            if ((void *)(tcp + 1) > data_end)
                return TC_ACT_OK;
            dport = ntohs(tcp->dest);
        } else if (proto == IPPROTO_UDP) {
            struct udphdr *udp = (void *)ip + (ip->ihl * 4);
            if ((void *)(udp + 1) > data_end)
                return TC_ACT_OK;
            dport = ntohs(udp->dest);
        } else {
            return TC_ACT_OK;
        }
        
        // Update map
        struct port_key key = {
            .port = dport,
            .protocol = proto,
            .direction = 0, // RX
        };
        
        struct port_stats *stats = bpf_map_lookup_elem(&port_stats_map, &key);
        if (stats) {
            __sync_fetch_and_add(&stats->bytes, skb->len);
            __sync_fetch_and_add(&stats->packets, 1);
        } else {
            struct port_stats new_stats = {
                .bytes = skb->len,
                .packets = 1,
            };
            bpf_map_update_elem(&port_stats_map, &key, &new_stats, BPF_ANY);
        }
    }
    
    // TODO: IPv6 handling (similar logic with ipv6hdr)
    
    return TC_ACT_OK;
}
```

### Egress (TX)
Similar to ingress, but:
- Use `sport` instead of `dport`
- Set `direction = 1`

## Go Integration

### Using cilium/ebpf

```go
package netstat

import (
    "github.com/cilium/ebpf"
    "github.com/cilium/ebpf/link"
)

type EBPFCollector struct {
    objs     *bpfObjects
    links    []link.Link
    statsMap *ebpf.Map
}

func NewEBPFCollector() (*EBPFCollector, error) {
    // Load pre-compiled BPF object
    objs := &bpfObjects{}
    if err := loadBpfObjects(objs, nil); err != nil {
        return nil, err
    }
    
    // Attach to interface
    iface, err := net.InterfaceByName("eth0") // TODO: all interfaces
    if err != nil {
        objs.Close()
        return nil, err
    }
    
    // Attach TC ingress
    linkIngress, err := link.AttachTCX(link.TCXOptions{
        Program:   objs.TcIngress,
        Attach:    ebpf.AttachTCXIngress,
        Interface: iface.Index,
    })
    if err != nil {
        objs.Close()
        return nil, err
    }
    
    // Attach TC egress
    linkEgress, err := link.AttachTCX(link.TCXOptions{
        Program:   objs.TcEgress,
        Attach:    ebpf.AttachTCXEgress,
        Interface: iface.Index,
    })
    if err != nil {
        linkIngress.Close()
        objs.Close()
        return nil, err
    }
    
    return &EBPFCollector{
        objs:     objs,
        links:    []link.Link{linkIngress, linkEgress},
        statsMap: objs.PortStatsMap,
    }, nil
}

func (e *EBPFCollector) Collect() ([]PortBytes, error) {
    var results []PortBytes
    
    var key PortKey
    var val PortStats
    
    iter := e.statsMap.Iterate()
    for iter.Next(&key, &val) {
        pb := PortBytes{
            Port:      key.Port,
            Protocol:  protocolName(key.Protocol),
            TS:        time.Now().UTC(),
        }
        
        if key.Direction == 0 {
            pb.RxBytes = val.Bytes
            pb.RxPackets = val.Packets
        } else {
            pb.TxBytes = val.Bytes
            pb.TxPackets = val.Packets
        }
        
        results = append(results, pb)
    }
    
    if err := iter.Err(); err != nil {
        return nil, err
    }
    
    return results, nil
}

func (e *EBPFCollector) Close() error {
    for _, l := range e.links {
        l.Close()
    }
    return e.objs.Close()
}
```

## Build Process

### Using bpf2go (cilium/ebpf)

```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -type port_key -type port_stats bpf portbytes.c -- -I/usr/include/bpf -O2 -g
```

This generates:
- `bpf_bpfel.go` / `bpf_bpfeb.go` (little/big endian)
- `bpf_bpfel.o` / `bpf_bpfeb.o` (embedded BPF bytecode)

### C Source
Place BPF program in `portbytes.c` alongside `portbytes.go`.

## Deployment

### Capabilities Required
- CAP_BPF (kernel 5.8+)
- Or CAP_SYS_ADMIN (older kernels)

### Systemd Unit
```ini
[Service]
AmbientCapabilities=CAP_BPF CAP_NET_ADMIN
```

### Container
```dockerfile
docker run --privileged ...
# or
docker run --cap-add=BPF --cap-add=NET_ADMIN ...
```

## Performance Considerations

1. **Map Size:**
   - 64K entries = ~1MB memory (depending on value size)
   - Use LRU map type to auto-evict old ports

2. **Per-Packet Overhead:**
   - TC hook adds ~100ns per packet
   - Map lookup/update: ~50ns
   - Total: ~150ns (negligible at <10Gbps)

3. **Concurrency:**
   - Use `__sync_fetch_and_add()` for atomic updates
   - BPF verifier enforces safety

## Delta Calculation

eBPF counters are cumulative. Go collector must:
1. Store previous values
2. Calculate deltas on each `Collect()`
3. Handle counter resets (program reload)

```go
type eBPFCache struct {
    prev map[PortKey]PortStats
}

func (e *EBPFCollector) CollectDeltas() ([]PortBytes, error) {
    current := // read from BPF map
    
    results := make([]PortBytes, 0)
    for key, cur := range current {
        prev, ok := e.cache.prev[key]
        if !ok {
            prev = PortStats{} // first time seeing this port
        }
        
        results = append(results, PortBytes{
            Port:      key.Port,
            RxBytes:   cur.RxBytes - prev.RxBytes,
            TxBytes:   cur.TxBytes - prev.TxBytes,
            // ...
        })
    }
    
    e.cache.prev = current
    return results, nil
}
```

## IPv6 Support

Add separate handling for ETH_P_IPV6:
```c
if (eth->h_proto == htons(ETH_P_IPV6)) {
    struct ipv6hdr *ip6 = data + sizeof(*eth);
    // Similar TCP/UDP header parsing
    // Use ip6->nexthdr for protocol
}
```

## Testing

### Local Testing
```bash
# Compile BPF program
clang -O2 -target bpf -c portbytes.c -o portbytes.o

# Load manually
sudo bpftool prog load portbytes.o /sys/fs/bpf/portbytes

# Attach to interface
sudo tc qdisc add dev eth0 clsact
sudo tc filter add dev eth0 ingress bpf da obj portbytes.o sec tc/ingress

# Verify
sudo bpftool map dump name port_stats_map

# Cleanup
sudo tc filter del dev eth0 ingress
sudo tc qdisc del dev eth0 clsact
```

### Go Test
```go
func TestEBPFCollector(t *testing.T) {
    if os.Getuid() != 0 {
        t.Skip("requires root")
    }
    
    collector, err := NewEBPFCollector()
    if err != nil {
        t.Fatal(err)
    }
    defer collector.Close()
    
    // Generate traffic
    conn, _ := net.Dial("tcp", "example.com:80")
    defer conn.Close()
    
    time.Sleep(100 * time.Millisecond)
    
    stats, err := collector.Collect()
    if err != nil {
        t.Fatal(err)
    }
    
    // Should see traffic on port 80
    found := false
    for _, s := range stats {
        if s.Port == 80 && s.TxBytes > 0 {
            found = true
            break
        }
    }
    
    if !found {
        t.Error("expected traffic on port 80")
    }
}
```

## References

- [cilium/ebpf documentation](https://ebpf-go.dev/)
- [BPF and XDP Reference Guide](https://docs.cilium.io/en/latest/bpf/)
- [Linux TC documentation](https://man7.org/linux/man-pages/man8/tc-bpf.8.html)
- [BPF CO-RE](https://nakryiko.com/posts/bpf-portability-and-co-re/)
