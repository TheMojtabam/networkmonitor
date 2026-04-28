//go:build !linux

package portbytes

func newEBPF(iface string) (Collector, error) {
	return nil, ErrNotSupported
}
