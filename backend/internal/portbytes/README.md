# Per-port bytes (eBPF)

This module counts RX/TX bytes per L4 port for TCP+UDP using eBPF attached at TC ingress/egress.

## Build requirements

- Linux kernel with eBPF support
- CAP_NET_ADMIN and CAP_BPF (or CAP_SYS_ADMIN on older kernels)
- clang + llvm

## Generate eBPF Go bindings

From `backend/`:

```bash
go generate ./...
```

This runs `bpf2go` to compile `ebpf/port_counter.c` into Go.

## Notes

- This is intentionally minimal.
- If eBPF is not available, the API should return a structured error.
