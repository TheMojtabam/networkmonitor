//go:build linux && ebpf_generated

package ebpf

// This file adapts the bpf2go-generated bindings (portcounter_x86_bpfel.go,
// portcounter_arm64_bpfel.go), which are produced by `go generate ./...`.
//
// With the `bpf2go ... PortCounter ...` invocation in collector.go, the
// generator emits PascalCase identifiers:
//
//   - type PortCounterObjects struct { ... }
//   - func LoadPortCounterObjects(obj any, opts *ebpf.CollectionOptions) error
//
// We expose them under the unexported name loadObjects() that load_linux.go
// uses, so the load_linux.go file doesn't need to know whether it's running
// against the real bindings or the stub.

func loadObjects() (*PortCounterObjects, func() error, error) {
	objs := &PortCounterObjects{}
	if err := LoadPortCounterObjects(objs, nil); err != nil {
		return nil, nil, err
	}
	return objs, objs.Close, nil
}
