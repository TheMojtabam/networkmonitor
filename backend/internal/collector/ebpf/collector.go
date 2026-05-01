// Package ebpf loads the port_counter eBPF program and exposes per-port
// byte counters from kernelspace.
//
// Build: requires `clang` and `bpf2go`. From this directory, run
//
//	go generate ./...
//
// to produce the Go bindings (portcounter_bpf*.go). Without the bindings
// the loader returns ErrNotSupported and callers fall back to /proc-based
// sampling.
//
// Runtime requirements: CAP_BPF + CAP_NET_ADMIN (or root) and a kernel
// that supports XDP generic mode (>=4.18 in practice).
package ebpf

import (
	"errors"

	"github.com/cilium/ebpf"

	t "github.com/mojtaba/portsleuth/backend/internal/collector"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall" -tags ebpf_generated -target amd64,arm64 PortCounter ../../../ebpf/port_counter.c -- -I../../../ebpf

// ErrNotSupported indicates eBPF couldn't be loaded — callers should fall back.
var ErrNotSupported = errors.New("eBPF not available")

// Collector wraps a loaded eBPF program attached to a single interface.
type Collector struct {
	iface     string
	supported bool

	// portMap is the kernel BPF map for per-port counters. Set on Linux only.
	portMap *ebpf.Map

	// closeFns are detach/close callbacks invoked in reverse order on Close().
	closeFns []func() error
}

// New attempts to load and attach the eBPF program.
// Returns ErrNotSupported on any failure.
func New(ifaceName string) (*Collector, error) {
	c := &Collector{iface: ifaceName}
	if err := c.load(); err != nil {
		return nil, err
	}
	return c, nil
}

// CollectTop returns up to limit per-port byte counters, sorted by total bytes.
func (c *Collector) CollectTop(limit int) ([]t.PortBytes, error) {
	if !c.supported {
		return nil, ErrNotSupported
	}
	return c.readMap(limit)
}

// Close detaches the eBPF program and frees resources.
func (c *Collector) Close() error {
	var firstErr error
	for i := len(c.closeFns) - 1; i >= 0; i-- {
		if err := c.closeFns[i](); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	c.closeFns = nil
	c.supported = false
	return firstErr
}

// Iface returns the interface this collector is attached to.
func (c *Collector) Iface() string { return c.iface }
