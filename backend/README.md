# PortSleuth Backend

Linux-focused network monitoring backend with comprehensive port and interface statistics.

## Features

### ✅ Implemented

1. **Interface Statistics** (`/api/net/interfaces`)
   - Per-interface RX/TX bytes, packets, errors, drops
   - Automatic rate calculation (bytes/sec, packets/sec)
   - Data source: `/proc/net/dev`

2. **Listening Ports** (`/api/net/ports`)
   - TCP/UDP listening ports (IPv4 + IPv6)
   - Connection counts per port
   - Data source: `/proc/net/{tcp,tcp6,udp,udp6}`

3. **Active Connections** (`/api/net/connections`)
   - All active TCP connections with state
   - Local and remote addresses
   - Data source: `/proc/net/tcp{,6}`

4. **Alternative Port Discovery** (`/api/net/ports-ss`)
   - Fallback using `ss` command
   - Works when `/proc` access is restricted

### 🚧 Partial Implementation

5. **Per-Port Byte Accounting** (`/api/net/port-bytes`)
   - Architecture ready for eBPF or nftables
   - Requires additional setup (see below)

## API Endpoints

### GET /api/health
Health check
```json
{"ok": true, "ts": "2026-04-28T15:30:00Z"}
```

### GET /api/sys
System information (CPU, memory, uptime)

### GET /api/net/interfaces
Interface statistics with rates
```json
{
  "interfaces": [
    {
      "name": "eth0",
      "rxBytes": 123456789,
      "txBytes": 987654321,
      "rxPackets": 12345,
      "txPackets": 9876,
      "ts": "2026-04-28T15:30:00Z"
    }
  ],
  "rates": [
    {
      "name": "eth0",
      "rxBytesPerSec": 1024000.5,
      "txBytesPerSec": 512000.2,
      "interval": 1.002,
      "ts": "2026-04-28T15:30:00Z"
    }
  ]
}
```

### GET /api/net/ports
Listening ports with connection counts
```json
{
  "ports": [
    {
      "protocol": "tcp",
      "localAddr": "0.0.0.0:80",
      "localPort": 80,
      "state": 10,
      "connectionCount": 42,
      "ts": "2026-04-28T15:30:00Z"
    }
  ]
}
```

### GET /api/net/connections
Active connections
```json
{
  "connections": [
    {
      "protocol": "tcp",
      "localAddr": "192.168.1.100:80",
      "remoteAddr": "192.168.1.50:54321",
      "state": 1,
      "ts": "2026-04-28T15:30:00Z"
    }
  ]
}
```

## Building

```bash
cd backend
go build -o portsleuthd ./cmd/portsleuthd
```

## Running

```bash
# Default: listen on :1234
./portsleuthd

# Custom port
./portsleuthd -listen :8080
```

## Per-Port Byte Accounting Setup

The `/api/net/port-bytes` endpoint requires additional setup for byte-level traffic accounting per port.

### Option 1: eBPF (Recommended)

**Requirements:**
- Linux kernel 4.18+ with eBPF support
- `bpftool` installed
- CAP_BPF or CAP_SYS_ADMIN capability

**Implementation needed:**
1. Write BPF program to hook network stack (XDP/TC)
2. Use BPF maps to track bytes per port
3. Load program on startup
4. Query maps in `Collect()`

**Libraries:**
- [cilium/ebpf](https://github.com/cilium/ebpf) - Pure Go eBPF
- [libbpfgo](https://github.com/aquasecurity/libbpfgo) - Go bindings for libbpf

**Stub location:** `internal/netstat/portbytes.go:EBPFCollector`

### Option 2: Nftables

**Requirements:**
- `nft` command available
- CAP_NET_ADMIN capability

**Setup:**
```bash
# Create accounting table
sudo nft add table inet portsleuth_accounting

# Add chains
sudo nft add chain inet portsleuth_accounting input \
  { type filter hook input priority 0 \; }
sudo nft add chain inet portsleuth_accounting output \
  { type filter hook output priority 0 \; }

# Add per-port counters (example for port 80)
sudo nft add rule inet portsleuth_accounting input \
  tcp dport 80 counter
sudo nft add rule inet portsleuth_accounting output \
  tcp sport 80 counter
```

**Dynamic rule management needed:**
1. Discover active ports via `/proc/net/tcp*`
2. Create counter rules for each port
3. Periodically read counters: `nft -j list table inet portsleuth_accounting`
4. Calculate deltas

**Stub location:** `internal/netstat/portbytes.go:NftablesCollector`

### Option 3: Conntrack (Alternative)

Use `conntrack` command for connection-level accounting:
```bash
sudo apt install conntrack
conntrack -L -o extended
```

Parse output for per-connection byte counts, aggregate by port.

## Rate Calculation

Interface rates are calculated automatically:
- First request: returns stats only (no rates yet)
- Subsequent requests: compares with previous snapshot
- Minimum interval: 1 second (configurable in `Cache`)

The cache is global per server instance.

## State Codes

TCP states (`PortInfo.State`):
- 1: ESTABLISHED
- 2: SYN_SENT
- 3: SYN_RECV
- 10: LISTEN

UDP sockets are always in state 7 (CLOSE) but filtered as listening.

## Security Considerations

1. **Privilege Requirements:**
   - Basic interface/port stats: no special privileges
   - Per-port bytes (eBPF): CAP_BPF or CAP_SYS_ADMIN
   - Per-port bytes (nftables): CAP_NET_ADMIN

2. **Information Exposure:**
   - API exposes listening ports and connections
   - Should run behind authentication in production
   - Consider firewall rules to restrict access

3. **Resource Usage:**
   - `/proc` parsing is efficient but scales with connection count
   - eBPF is lowest overhead
   - Nftables rule count should be managed (don't create thousands)

## Dependencies

```go.mod
module github.com/mojtaba/portsleuth/backend

go 1.22

// Optional (for eBPF implementation):
// require github.com/cilium/ebpf v0.12.0
```

## Project Structure

```
backend/
├── cmd/
│   └── portsleuthd/
│       └── main.go              # HTTP server + routes
├── internal/
│   ├── sysinfo/
│   │   └── sysinfo.go          # System metrics
│   └── netstat/
│       ├── interfaces.go        # Interface stats + rates
│       ├── ports.go             # Port listing + connections
│       ├── portbytes.go         # Per-port byte accounting (eBPF/nftables)
│       └── cache.go             # Rate calculation cache
├── go.mod
└── README.md
```

## Testing

```bash
# Start server
./portsleuthd

# Test endpoints
curl http://localhost:1234/api/health
curl http://localhost:1234/api/net/interfaces
curl http://localhost:1234/api/net/ports
curl http://localhost:1234/api/net/connections

# Generate traffic and test rates
curl http://localhost:1234/api/net/interfaces
sleep 2
curl http://localhost:1234/api/net/interfaces  # Should show rates
```

## Future Enhancements

1. **eBPF Implementation:**
   - Complete BPF program for per-port byte tracking
   - CO-RE (Compile Once, Run Everywhere) support

2. **WebSocket Support:**
   - Real-time streaming of stats updates
   - Server-sent events for live dashboards

3. **Historical Data:**
   - Optional time-series storage (SQLite/InfluxDB)
   - Retention policies

4. **Process Correlation:**
   - Match ports to processes (via `/proc/[pid]/fd/`)
   - Requires CAP_DAC_READ_SEARCH or same UID

5. **IPv6 Support:**
   - Already parsing tcp6/udp6, but address display can be improved
   - Better IPv6 address formatting

## License

MIT
