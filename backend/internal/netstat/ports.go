package netstat

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// PortState represents state of a listening port or connection
type PortState int

const (
	StateEstablished PortState = 1
	StateSynSent     PortState = 2
	StateSynRecv     PortState = 3
	StateFinWait1    PortState = 4
	StateFinWait2    PortState = 5
	StateTimeWait    PortState = 6
	StateClose       PortState = 7
	StateCloseWait   PortState = 8
	StateLastAck     PortState = 9
	StateListen      PortState = 10
	StateClosing     PortState = 11
)

// PortInfo represents a listening port with connection counts
type PortInfo struct {
	Protocol       string    `json:"protocol"`       // tcp, tcp6, udp, udp6
	LocalAddr      string    `json:"localAddr"`      // IP:port
	LocalPort      uint16    `json:"localPort"`
	State          PortState `json:"state"`
	ConnectionCount int      `json:"connectionCount"` // For TCP listen sockets
	TS             time.Time `json:"ts"`
}

// ConnectionInfo represents an active connection
type ConnectionInfo struct {
	Protocol   string    `json:"protocol"`
	LocalAddr  string    `json:"localAddr"`
	RemoteAddr string    `json:"remoteAddr"`
	State      PortState `json:"state"`
	TS         time.Time `json:"ts"`
}

// CollectListeningPorts scans /proc/net/{tcp,tcp6,udp,udp6} for listening ports
func CollectListeningPorts() ([]PortInfo, error) {
	now := time.Now().UTC()
	var allPorts []PortInfo

	// TCP IPv4
	tcp4, err := parseProcNet("/proc/net/tcp", "tcp", now)
	if err == nil {
		allPorts = append(allPorts, filterListening(tcp4)...)
	}

	// TCP IPv6
	tcp6, err := parseProcNet("/proc/net/tcp6", "tcp6", now)
	if err == nil {
		allPorts = append(allPorts, filterListening(tcp6)...)
	}

	// UDP IPv4
	udp4, err := parseProcNet("/proc/net/udp", "udp", now)
	if err == nil {
		allPorts = append(allPorts, filterListening(udp4)...)
	}

	// UDP IPv6
	udp6, err := parseProcNet("/proc/net/udp6", "udp6", now)
	if err == nil {
		allPorts = append(allPorts, filterListening(udp6)...)
	}

	return allPorts, nil
}

// CollectConnections returns all active connections (for counting per port)
func CollectConnections() ([]ConnectionInfo, error) {
	now := time.Now().UTC()
	var allConns []ConnectionInfo

	// TCP IPv4
	tcp4, err := parseConnections("/proc/net/tcp", "tcp", now)
	if err == nil {
		allConns = append(allConns, tcp4...)
	}

	// TCP IPv6
	tcp6, err := parseConnections("/proc/net/tcp6", "tcp6", now)
	if err == nil {
		allConns = append(allConns, tcp6...)
	}

	return allConns, nil
}

// CountConnectionsPerPort groups connections by local port
func CountConnectionsPerPort(ports []PortInfo, conns []ConnectionInfo) []PortInfo {
	// Build connection count map by localAddr
	countMap := make(map[string]int)
	for _, conn := range conns {
		if conn.State == StateEstablished || conn.State == StateSynRecv {
			countMap[conn.LocalAddr]++
		}
	}

	// Update port info with counts
	result := make([]PortInfo, len(ports))
	copy(result, ports)

	for i := range result {
		if count, ok := countMap[result[i].LocalAddr]; ok {
			result[i].ConnectionCount = count
		}
	}

	return result
}

func parseProcNet(path, protocol string, ts time.Time) ([]PortInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var ports []PortInfo
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum == 1 {
			continue // skip header
		}

		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}

		localAddr := fields[1]
		state := parseHexInt(fields[3])

		ip, port := parseAddress(localAddr)

		ports = append(ports, PortInfo{
			Protocol:  protocol,
			LocalAddr: fmt.Sprintf("%s:%d", ip, port),
			LocalPort: port,
			State:     PortState(state),
			TS:        ts,
		})
	}

	return ports, scanner.Err()
}

func parseConnections(path, protocol string, ts time.Time) ([]ConnectionInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var conns []ConnectionInfo
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum == 1 {
			continue
		}

		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}

		localAddr := fields[1]
		remoteAddr := fields[2]
		state := parseHexInt(fields[3])

		localIP, localPort := parseAddress(localAddr)
		remoteIP, remotePort := parseAddress(remoteAddr)

		conns = append(conns, ConnectionInfo{
			Protocol:   protocol,
			LocalAddr:  fmt.Sprintf("%s:%d", localIP, localPort),
			RemoteAddr: fmt.Sprintf("%s:%d", remoteIP, remotePort),
			State:      PortState(state),
			TS:         ts,
		})
	}

	return conns, scanner.Err()
}

func filterListening(ports []PortInfo) []PortInfo {
	var result []PortInfo
	for _, p := range ports {
		// TCP: state == LISTEN (10)
		// UDP: always considered "listening" (state 07)
		if p.State == StateListen || strings.HasPrefix(p.Protocol, "udp") {
			result = append(result, p)
		}
	}
	return result
}

// parseAddress decodes hex-encoded IP:port from /proc/net/*
// Example: "0100007F:0050" -> 127.0.0.1:80
func parseAddress(hexAddr string) (string, uint16) {
	parts := strings.Split(hexAddr, ":")
	if len(parts) != 2 {
		return "0.0.0.0", 0
	}

	ipHex := parts[0]
	portHex := parts[1]

	port := uint16(parseHexInt(portHex))
	ip := decodeIP(ipHex)

	return ip, port
}

func decodeIP(hexStr string) string {
	data, err := hex.DecodeString(hexStr)
	if err != nil || len(data) == 0 {
		return "0.0.0.0"
	}

	if len(data) == 4 {
		// IPv4 (little-endian)
		return net.IPv4(data[3], data[2], data[1], data[0]).String()
	}

	// IPv6 (little-endian per 4-byte word)
	if len(data) == 16 {
		ip := make(net.IP, 16)
		for i := 0; i < 4; i++ {
			base := i * 4
			ip[base] = data[base+3]
			ip[base+1] = data[base+2]
			ip[base+2] = data[base+1]
			ip[base+3] = data[base]
		}
		return ip.String()
	}

	return "unknown"
}

func parseHexInt(s string) int {
	v, _ := strconv.ParseInt(s, 16, 64)
	return int(v)
}
