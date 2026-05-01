// Package procnet parses /proc/net/{tcp,tcp6,udp,udp6} and attributes
// sockets to processes via /proc/[pid]/fd/* inode lookups.
package procnet

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	t "github.com/mojtaba/portsleuth/backend/internal/collector"
)

// ScanListening returns all ports currently in LISTEN state (TCP) plus
// every UDP socket (UDP doesn't have a separate listening state).
func ScanListening() ([]t.Port, error) {
	now := time.Now().UTC()
	var out []t.Port

	for _, src := range []struct {
		path  string
		proto t.Protocol
	}{
		{"/proc/net/tcp", t.ProtoTCP},
		{"/proc/net/tcp6", t.ProtoTCP6},
		{"/proc/net/udp", t.ProtoUDP},
		{"/proc/net/udp6", t.ProtoUDP6},
	} {
		ports, _ := parsePorts(src.path, src.proto, now)
		out = append(out, filterListening(ports)...)
	}
	return out, nil
}

// ScanAll returns ALL sockets including non-listening (used for connection counts).
func ScanAll() ([]t.Port, error) {
	now := time.Now().UTC()
	var out []t.Port
	for _, src := range []struct {
		path  string
		proto t.Protocol
	}{
		{"/proc/net/tcp", t.ProtoTCP},
		{"/proc/net/tcp6", t.ProtoTCP6},
		{"/proc/net/udp", t.ProtoUDP},
		{"/proc/net/udp6", t.ProtoUDP6},
	} {
		ports, _ := parsePorts(src.path, src.proto, now)
		out = append(out, ports...)
	}
	return out, nil
}

// ScanConnections returns Connection rows (local + remote endpoints).
func ScanConnections() ([]t.Connection, error) {
	now := time.Now().UTC()
	var out []t.Connection
	for _, src := range []struct {
		path  string
		proto t.Protocol
	}{
		{"/proc/net/tcp", t.ProtoTCP},
		{"/proc/net/tcp6", t.ProtoTCP6},
		{"/proc/net/udp", t.ProtoUDP},
		{"/proc/net/udp6", t.ProtoUDP6},
	} {
		conns, _ := parseConnections(src.path, src.proto, now)
		out = append(out, conns...)
	}
	return out, nil
}

// CountConnsPerPort fills ConnectionCount based on a connection list.
// Only ESTABLISHED connections count.
func CountConnsPerPort(ports []t.Port, conns []t.Connection) []t.Port {
	counts := make(map[string]int)
	for _, c := range conns {
		if c.State == t.StateEstablished {
			counts[c.LocalAddr]++
		}
	}
	for i := range ports {
		if n, ok := counts[ports[i].LocalAddr]; ok {
			ports[i].ConnectionCount = n
		}
	}
	return ports
}

// ----------- low-level parsers -----------

type rawSock struct {
	localIP    string
	localPort  uint16
	remoteIP   string
	remotePort uint16
	state      t.PortState
	inode      string
}

func parseRaw(path string, ts time.Time) ([]rawSock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []rawSock
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<16), 1<<20)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		lip, lport := parseAddr(fields[1])
		rip, rport := parseAddr(fields[2])
		state := t.PortState(parseHexInt(fields[3]))
		inode := fields[9]
		out = append(out, rawSock{lip, lport, rip, rport, state, inode})
	}
	return out, scanner.Err()
}

func parsePorts(path string, proto t.Protocol, ts time.Time) ([]t.Port, error) {
	raw, err := parseRaw(path, ts)
	if err != nil {
		return nil, err
	}
	out := make([]t.Port, 0, len(raw))
	for _, r := range raw {
		out = append(out, t.Port{
			Protocol:  proto,
			LocalAddr: fmt.Sprintf("%s:%d", r.localIP, r.localPort),
			LocalIP:   r.localIP,
			LocalPort: r.localPort,
			State:     r.state,
			StateName: r.state.String(),
			TS:        ts,
		})
	}
	return out, nil
}

func parseConnections(path string, proto t.Protocol, ts time.Time) ([]t.Connection, error) {
	raw, err := parseRaw(path, ts)
	if err != nil {
		return nil, err
	}
	out := make([]t.Connection, 0, len(raw))
	for _, r := range raw {
		out = append(out, t.Connection{
			Protocol:   proto,
			LocalAddr:  fmt.Sprintf("%s:%d", r.localIP, r.localPort),
			RemoteAddr: fmt.Sprintf("%s:%d", r.remoteIP, r.remotePort),
			LocalIP:    r.localIP,
			LocalPort:  r.localPort,
			RemoteIP:   r.remoteIP,
			RemotePort: r.remotePort,
			State:      r.state,
			StateName:  r.state.String(),
			TS:         ts,
		})
	}
	return out, nil
}

func filterListening(ports []t.Port) []t.Port {
	out := ports[:0]
	for _, p := range ports {
		if p.State == t.StateListen || strings.HasPrefix(string(p.Protocol), "udp") {
			out = append(out, p)
		}
	}
	return out
}

// parseAddr decodes "0100007F:0050" → ("127.0.0.1", 80).
func parseAddr(hexAddr string) (string, uint16) {
	parts := strings.Split(hexAddr, ":")
	if len(parts) != 2 {
		return "0.0.0.0", 0
	}
	port := uint16(parseHexInt(parts[1]))
	ip := decodeIP(parts[0])
	return ip, port
}

func decodeIP(hexStr string) string {
	data, err := hex.DecodeString(hexStr)
	if err != nil || len(data) == 0 {
		return "0.0.0.0"
	}
	if len(data) == 4 {
		return net.IPv4(data[3], data[2], data[1], data[0]).String()
	}
	if len(data) == 16 {
		ip := make(net.IP, 16)
		for i := 0; i < 4; i++ {
			b := i * 4
			ip[b] = data[b+3]
			ip[b+1] = data[b+2]
			ip[b+2] = data[b+1]
			ip[b+3] = data[b]
		}
		return ip.String()
	}
	return "unknown"
}

func parseHexInt(s string) int {
	v, _ := strconv.ParseInt(s, 16, 64)
	return int(v)
}

// ============================================================
// Process attribution: map socket inode → PID + cmdline
// ============================================================

// ProcResolver is a cached inode→process resolver. The /proc/[pid]/fd/*
// scan is expensive, so we cache results between calls and refresh
// periodically. Safe for concurrent use.
type ProcResolver struct {
	mu      sync.RWMutex
	cache   map[string]ProcInfo // inode → info
	lastRun time.Time
	ttl     time.Duration
}

// ProcInfo is the resolved process for a socket.
type ProcInfo struct {
	PID  int
	Name string
}

// NewProcResolver returns a resolver that refreshes its cache every ttl.
func NewProcResolver(ttl time.Duration) *ProcResolver {
	return &ProcResolver{
		cache: map[string]ProcInfo{},
		ttl:   ttl,
	}
}

// Refresh re-scans /proc to rebuild the inode → PID map.
func (r *ProcResolver) Refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Since(r.lastRun) < r.ttl {
		return
	}
	cache := map[string]ProcInfo{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fdDir := filepath.Join("/proc", e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // permission denied for processes we don't own
		}
		var name string
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			// "socket:[12345]" → "12345"
			if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
				inode := link[len("socket:[") : len(link)-1]
				if name == "" {
					name = readProcessName(pid)
				}
				cache[inode] = ProcInfo{PID: pid, Name: name}
			}
		}
	}
	r.cache = cache
	r.lastRun = time.Now()
}

// Lookup returns process info for the given socket inode, or zero value if unknown.
func (r *ProcResolver) Lookup(inode string) ProcInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cache[inode]
}

// AttributePorts fills Process and PID for every port. It calls Refresh()
// internally if the cache is stale.
func (r *ProcResolver) AttributePorts(ports []t.Port) []t.Port {
	r.Refresh()
	// Re-parse /proc/net/* to get inodes (they're not on Port struct)
	inodeMap := r.collectInodeMap()
	for i := range ports {
		key := fmt.Sprintf("%s|%s", ports[i].Protocol, ports[i].LocalAddr)
		if inode, ok := inodeMap[key]; ok {
			info := r.Lookup(inode)
			ports[i].PID = info.PID
			ports[i].Process = info.Name
		}
	}
	return ports
}

// AttributeConnections fills Process and PID on connections.
func (r *ProcResolver) AttributeConnections(conns []t.Connection) []t.Connection {
	r.Refresh()
	inodeMap := r.collectInodeMap()
	for i := range conns {
		key := fmt.Sprintf("%s|%s", conns[i].Protocol, conns[i].LocalAddr)
		if inode, ok := inodeMap[key]; ok {
			info := r.Lookup(inode)
			conns[i].PID = info.PID
			conns[i].Process = info.Name
		}
	}
	return conns
}

// collectInodeMap rebuilds a "proto|addr" → inode map by re-scanning /proc/net.
// Cheap because /proc/net files are small.
func (r *ProcResolver) collectInodeMap() map[string]string {
	out := map[string]string{}
	for _, src := range []struct {
		path  string
		proto t.Protocol
	}{
		{"/proc/net/tcp", t.ProtoTCP},
		{"/proc/net/tcp6", t.ProtoTCP6},
		{"/proc/net/udp", t.ProtoUDP},
		{"/proc/net/udp6", t.ProtoUDP6},
	} {
		raw, err := parseRaw(src.path, time.Now())
		if err != nil {
			continue
		}
		for _, s := range raw {
			key := fmt.Sprintf("%s|%s:%d", src.proto, s.localIP, s.localPort)
			out[key] = s.inode
		}
	}
	return out
}

func readProcessName(pid int) string {
	// Prefer /proc/[pid]/comm (truncated to 16 chars but reliable)
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}
