//go:build linux && !ebpf_generated

package ebpf

import (
	"fmt"

	"github.com/cilium/ebpf"
)

// PortCounterObjects mirrors the struct that bpf2go generates so that
// load_linux.go compiles even without the generated bindings present.
//
// To enable real eBPF: install clang/llvm + libbpf headers + linux-headers
// + gcc-multilib, then:
//
//	cd backend/internal/collector/ebpf && go generate ./...
//	go build -tags ebpf_generated ./...
type PortCounterObjects struct {
	XdpCountIngress *ebpf.Program
	TcCountEgress   *ebpf.Program
	PortCounters    *ebpf.Map
}

// Close is a no-op on the stub — there's nothing to close.
func (*PortCounterObjects) Close() error { return nil }

func loadObjects() (*PortCounterObjects, func() error, error) {
	return nil, nil, fmt.Errorf("eBPF bindings not generated — run `go generate ./...` in backend/internal/collector/ebpf and rebuild with -tags ebpf_generated")
}
