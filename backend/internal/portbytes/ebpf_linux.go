//go:build linux

package portbytes

import (
	"sort"
)

// Placeholder: to enable real eBPF collection, run `go generate ./...`
// after installing clang/llvm and ensure bpf2go generates port_counter_* files.

type ebpfCollector struct{}

func newEBPF(ifaceName string) (*ebpfCollector, error) {
	return nil, ErrNotSupported
}

func (c *ebpfCollector) Close() error { return nil }

func (c *ebpfCollector) CollectTop(limit int) ([]PortBytes, error) {
	_ = sort.Slice
	return nil, ErrNotSupported
}
