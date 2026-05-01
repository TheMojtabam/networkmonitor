// Package types holds shared data structures exchanged between collectors,
// the in-memory store, the API layer, and the WebSocket pump.
package types

import "time"

// Protocol is one of: tcp, tcp6, udp, udp6.
type Protocol string

const (
	ProtoTCP  Protocol = "tcp"
	ProtoTCP6 Protocol = "tcp6"
	ProtoUDP  Protocol = "udp"
	ProtoUDP6 Protocol = "udp6"
)

// PortState mirrors the kernel TCP states from /proc/net/tcp.
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

// String returns the canonical TCP state name.
func (s PortState) String() string {
	switch s {
	case StateEstablished:
		return "ESTABLISHED"
	case StateSynSent:
		return "SYN_SENT"
	case StateSynRecv:
		return "SYN_RECV"
	case StateFinWait1:
		return "FIN_WAIT1"
	case StateFinWait2:
		return "FIN_WAIT2"
	case StateTimeWait:
		return "TIME_WAIT"
	case StateClose:
		return "CLOSE"
	case StateCloseWait:
		return "CLOSE_WAIT"
	case StateLastAck:
		return "LAST_ACK"
	case StateListen:
		return "LISTEN"
	case StateClosing:
		return "CLOSING"
	}
	return "UNKNOWN"
}

// InterfaceStats is a snapshot of /proc/net/dev counters for one interface.
type InterfaceStats struct {
	Name      string    `json:"name"`
	RxBytes   uint64    `json:"rxBytes"`
	RxPackets uint64    `json:"rxPackets"`
	RxErrors  uint64    `json:"rxErrors"`
	RxDropped uint64    `json:"rxDropped"`
	TxBytes   uint64    `json:"txBytes"`
	TxPackets uint64    `json:"txPackets"`
	TxErrors  uint64    `json:"txErrors"`
	TxDropped uint64    `json:"txDropped"`
	TS        time.Time `json:"ts"`
}

// InterfaceRate is the per-second derivative of two InterfaceStats samples.
type InterfaceRate struct {
	Name          string    `json:"name"`
	RxBytesPerSec float64   `json:"rxBytesPerSec"`
	TxBytesPerSec float64   `json:"txBytesPerSec"`
	RxPktsPerSec  float64   `json:"rxPktsPerSec"`
	TxPktsPerSec  float64   `json:"txPktsPerSec"`
	TS            time.Time `json:"ts"`
}

// Port describes a single (proto, port, addr) endpoint.
type Port struct {
	Protocol        Protocol  `json:"protocol"`
	LocalAddr       string    `json:"localAddr"` // ip:port
	LocalIP         string    `json:"localIp"`
	LocalPort       uint16    `json:"localPort"`
	State           PortState `json:"state"`
	StateName       string    `json:"stateName"`
	Process         string    `json:"process,omitempty"`
	PID             int       `json:"pid,omitempty"`
	ConnectionCount int       `json:"connectionCount"`
	RxBytesPerSec   float64   `json:"rxBytesPerSec"`
	TxBytesPerSec   float64   `json:"txBytesPerSec"`
	TotalBps        float64   `json:"totalBps"` // rx + tx
	TS              time.Time `json:"ts"`
}

// Connection represents a single (local, remote) TCP/UDP flow.
type Connection struct {
	Protocol   Protocol  `json:"protocol"`
	LocalAddr  string    `json:"localAddr"`
	RemoteAddr string    `json:"remoteAddr"`
	LocalIP    string    `json:"localIp"`
	LocalPort  uint16    `json:"localPort"`
	RemoteIP   string    `json:"remoteIp"`
	RemotePort uint16    `json:"remotePort"`
	State      PortState `json:"state"`
	StateName  string    `json:"stateName"`
	Process    string    `json:"process,omitempty"`
	PID        int       `json:"pid,omitempty"`
	Country    string    `json:"country,omitempty"`
	ASN        string    `json:"asn,omitempty"`
	RxBytes    uint64    `json:"rxBytes,omitempty"`
	TxBytes    uint64    `json:"txBytes,omitempty"`
	Age        float64   `json:"age,omitempty"` // seconds
	TS         time.Time `json:"ts"`
}

// PortBytes is the raw eBPF (or fallback) per-port byte counter.
type PortBytes struct {
	Port      uint16 `json:"port"`
	Protocol  string `json:"protocol"`
	RxBytes   uint64 `json:"rxBytes"`
	TxBytes   uint64 `json:"txBytes"`
	RxPackets uint64 `json:"rxPackets"`
	TxPackets uint64 `json:"txPackets"`
}

// Snapshot is what we send over the WebSocket each tick.
type Snapshot struct {
	TS         time.Time        `json:"ts"`
	Interfaces []InterfaceStats `json:"interfaces"`
	Rates      []InterfaceRate  `json:"rates"`
	Ports      []Port           `json:"ports"`
	TopPorts   []Port           `json:"topPorts"` // top-N by total bps
	Totals     SnapshotTotals   `json:"totals"`
}

// SnapshotTotals are dashboard stat-card numbers.
type SnapshotTotals struct {
	RxBytesPerSec   float64 `json:"rxBytesPerSec"`
	TxBytesPerSec   float64 `json:"txBytesPerSec"`
	ListeningPorts  int     `json:"listeningPorts"`
	ActiveConns     int     `json:"activeConns"`
	EstablishedConn int     `json:"establishedConn"`
}

// HistoryPoint is one point in a time series.
type HistoryPoint struct {
	TS            time.Time `json:"ts"`
	RxBytesPerSec float64   `json:"rxBytesPerSec"`
	TxBytesPerSec float64   `json:"txBytesPerSec"`
}

// PortHistoryPoint is per-port time-series.
type PortHistoryPoint struct {
	TS       time.Time `json:"ts"`
	Port     uint16    `json:"port"`
	Protocol Protocol  `json:"protocol"`
	RxBps    float64   `json:"rxBps"`
	TxBps    float64   `json:"txBps"`
}
