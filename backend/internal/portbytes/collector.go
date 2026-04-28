package portbytes

import (
	"errors"
	"fmt"
	"os"
	"runtime"
)

var (
	ErrNotSupported = errors.New("per-port bytes requires eBPF (Linux) and sufficient capabilities")
)

type PortBytes struct {
	Port      uint16 `json:"port"`
	RxBytes   uint64 `json:"rxBytes"`
	TxBytes   uint64 `json:"txBytes"`
	RxPackets uint64 `json:"rxPackets"`
	TxPackets uint64 `json:"txPackets"`
}

type Collector interface {
	CollectTop(limit int) ([]PortBytes, error)
	Close() error
}

func New(iface string) (Collector, error) {
	if runtime.GOOS != "linux" {
		return nil, ErrNotSupported
	}
	// Quick check for required tools at build-time runtime.
	if _, err := os.Stat("/sys/fs/bpf"); err != nil {
		// still might work depending on setup, but this is a strong signal.
		_ = err
	}
	return newEBPF(iface)
}

func WrapErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("portbytes: %w", err)
}
