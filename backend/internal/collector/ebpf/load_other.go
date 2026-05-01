//go:build !linux

package ebpf

import (
	t "github.com/mojtaba/portsleuth/backend/internal/collector"
)

func (c *Collector) load() error { return ErrNotSupported }
func (c *Collector) readMap(int) ([]t.PortBytes, error) {
	return nil, ErrNotSupported
}
