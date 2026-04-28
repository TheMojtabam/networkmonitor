package netstat

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PortBytesCollector defines interface for per-port byte accounting
type PortBytesCollector interface {
	Collect() ([]PortBytes, error)
	Close() error
}

// PortBytes represents per-port traffic statistics
type PortBytes struct {
	Protocol  string    `json:"protocol"`  // tcp/udp
	Port      uint16    `json:"port"`
	RxBytes   uint64    `json:"rxBytes"`
	TxBytes   uint64    `json:"txBytes"`
	RxPackets uint64    `json:"rxPackets"`
	TxPackets uint64    `json:"txPackets"`
	TS        time.Time `json:"ts"`
}

// NewPortBytesCollector attempts eBPF first, falls back to nftables
func NewPortBytesCollector() (PortBytesCollector, error) {
	// Try eBPF (requires bpftool, kernel support, and privileges)
	if ebpfAvailable() {
		collector, err := NewEBPFCollector()
		if err == nil {
			return collector, nil
		}
	}

	// Fallback to nftables accounting
	return NewNftablesCollector()
}

// ebpfAvailable checks if eBPF tooling is present
func ebpfAvailable() bool {
	_, err := exec.LookPath("bpftool")
	return err == nil
}

// =============================================================================
// eBPF Collector (stub - requires actual BPF program)
// =============================================================================

type EBPFCollector struct {
	mu      sync.Mutex
	running bool
}

func NewEBPFCollector() (*EBPFCollector, error) {
	// TODO: Load BPF program that hooks into network stack
	// - Attach to tc (traffic control) or XDP hooks
	// - Track bytes per port using BPF maps
	// - Use cilium/ebpf or libbpfgo libraries
	
	return nil, errors.New("eBPF collector not yet implemented - requires BPF program compilation")
}

func (e *EBPFCollector) Collect() ([]PortBytes, error) {
	return nil, errors.New("not implemented")
}

func (e *EBPFCollector) Close() error {
	return nil
}

// =============================================================================
// Nftables Collector (accounting rules)
// =============================================================================

type NftablesCollector struct {
	mu          sync.Mutex
	initialized bool
	tableName   string
}

func NewNftablesCollector() (*NftablesCollector, error) {
	// Check if nft is available
	if _, err := exec.LookPath("nft"); err != nil {
		return nil, fmt.Errorf("nftables not available: %w", err)
	}

	nc := &NftablesCollector{
		tableName: "portsleuth_accounting",
	}

	// Initialize accounting rules
	if err := nc.initRules(); err != nil {
		return nil, err
	}

	return nc, nil
}

func (nc *NftablesCollector) initRules() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if nc.initialized {
		return nil
	}

	// Create accounting table and chains
	// This is a design/example - actual deployment requires root privileges
	script := fmt.Sprintf(`
# Create table
nft add table inet %s

# Input chain (count incoming packets per port)
nft add chain inet %s input { type filter hook input priority 0 \; }

# Output chain (count outgoing packets per port)
nft add chain inet %s output { type filter hook output priority 0 \; }

# Example: Add counter for specific ports (would be dynamic)
# nft add rule inet %s input tcp dport 80 counter
# nft add rule inet %s output tcp sport 80 counter
`, nc.tableName, nc.tableName, nc.tableName, nc.tableName, nc.tableName)

	// NOTE: This requires CAP_NET_ADMIN privileges
	// In production, this should be run during setup/initialization
	// For now, we'll just validate nft is available
	_ = script // Placeholder for actual setup

	nc.initialized = true
	return nil
}

func (nc *NftablesCollector) Collect() ([]PortBytes, error) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	// Query nftables counters
	cmd := exec.Command("nft", "-j", "list", "table", "inet", nc.tableName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list nftables: %w", err)
	}

	// Parse JSON output from nft
	// This is a simplified example - real implementation needs proper JSON parsing
	_ = output
	
	// For now, return empty slice as this requires:
	// 1. Root privileges to create rules
	// 2. Dynamic rule creation based on discovered ports
	// 3. Periodic counter reading and delta calculation
	
	return []PortBytes{}, nil
}

func (nc *NftablesCollector) Close() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if !nc.initialized {
		return nil
	}

	// Clean up nftables rules
	cmd := exec.Command("nft", "delete", "table", "inet", nc.tableName)
	_ = cmd.Run() // Best effort

	nc.initialized = false
	return nil
}

// =============================================================================
// SS-based fallback (connection-level, not byte-level)
// =============================================================================

// SSCollector uses 'ss' command to get socket statistics
type SSCollector struct{}

func NewSSCollector() *SSCollector {
	return &SSCollector{}
}

// CollectWithSS uses the 'ss' command as a fallback
func (s *SSCollector) CollectWithSS() ([]PortInfo, error) {
	// ss -tulpn: tcp, udp, listening, process info, numeric
	cmd := exec.Command("ss", "-tulpn")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ss command failed: %w", err)
	}

	return parseSSOutput(string(output)), nil
}

func parseSSOutput(output string) []PortInfo {
	var ports []PortInfo
	lines := strings.Split(output, "\n")
	now := time.Now().UTC()

	for _, line := range lines {
		if strings.HasPrefix(line, "Netid") || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		proto := fields[0]
		state := fields[1]
		localAddr := fields[4]

		// Parse local address
		parts := strings.Split(localAddr, ":")
		if len(parts) < 2 {
			continue
		}

		portStr := parts[len(parts)-1]
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			continue
		}

		var portState PortState
		if state == "LISTEN" || proto == "udp" {
			portState = StateListen
		}

		ports = append(ports, PortInfo{
			Protocol:  proto,
			LocalAddr: localAddr,
			LocalPort: uint16(port),
			State:     portState,
			TS:        now,
		})
	}

	return ports
}
