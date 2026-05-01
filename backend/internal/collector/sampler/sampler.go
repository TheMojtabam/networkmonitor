// Package sampler is the heart of the data pipeline. It runs collectors
// at configured intervals, computes rates from snapshot deltas, attributes
// processes via /proc, enriches with GeoIP, and publishes Snapshot
// messages to subscribers (the WebSocket pump and the alert engine).
package sampler

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/mojtaba/portsleuth/backend/internal/config"

	t "github.com/mojtaba/portsleuth/backend/internal/collector"
	"github.com/mojtaba/portsleuth/backend/internal/collector/ebpf"
	"github.com/mojtaba/portsleuth/backend/internal/collector/fallback"
	"github.com/mojtaba/portsleuth/backend/internal/collector/netstat"
	"github.com/mojtaba/portsleuth/backend/internal/collector/procnet"
	"github.com/mojtaba/portsleuth/backend/internal/store"
)

// PerPortSampler is anything that returns top-N per-port byte counters.
type PerPortSampler interface {
	CollectTop(limit int) ([]t.PortBytes, error)
	Close() error
}

// GeoEnricher is anything that fills Country/ASN on a connection.
type GeoEnricher interface {
	Enrich(ip string) (country, asn string)
}

// Logger is a minimal logging interface (matches log.Logger and slog.Logger).
type Logger interface {
	Printf(format string, args ...any)
}

// Sampler ticks the collection pipeline. After Start, the latest snapshot
// is always available via Latest(); subscribers receive each new snapshot
// over the channel returned by Subscribe.
type Sampler struct {
	cfg      config.CollectorConfig
	mem      *store.Memory
	procRes  *procnet.ProcResolver
	portSamp PerPortSampler
	geo      GeoEnricher
	log      Logger

	mu          sync.RWMutex
	latest      t.Snapshot
	prevIface   []t.InterfaceStats
	subscribers []chan t.Snapshot
}

// New wires up the sampler. portSamp may be nil — if so, top-port
// statistics will be empty until eBPF or fallback becomes available.
func New(cfg config.CollectorConfig, mem *store.Memory, geo GeoEnricher, log Logger) *Sampler {
	return &Sampler{
		cfg:     cfg,
		mem:     mem,
		geo:     geo,
		log:     log,
		procRes: procnet.NewProcResolver(5 * time.Second),
	}
}

// SetPortSampler is called once after the sampler is created — first try
// eBPF, then fall back to ss-based on failure.
func (s *Sampler) SetPortSampler(p PerPortSampler) {
	s.mu.Lock()
	s.portSamp = p
	s.mu.Unlock()
}

// Subscribe returns a channel that receives every new snapshot.
// The buffer is small; slow consumers will drop snapshots.
func (s *Sampler) Subscribe() <-chan t.Snapshot {
	ch := make(chan t.Snapshot, 4)
	s.mu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.mu.Unlock()
	return ch
}

// Latest returns the most recent snapshot.
func (s *Sampler) Latest() t.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest
}

// Run starts the sampling loop. Blocks until ctx is cancelled.
func (s *Sampler) Run(ctx context.Context) {
	tick := time.NewTicker(s.cfg.InterfaceInterval())
	defer tick.Stop()

	// First sample immediately to establish baseline.
	s.tickOnce()

	for {
		select {
		case <-ctx.Done():
			s.cleanup()
			return
		case <-tick.C:
			s.tickOnce()
		}
	}
}

func (s *Sampler) tickOnce() {
	now := time.Now().UTC()

	// 1. Interfaces
	currIface, err := netstat.Collect()
	if err != nil {
		s.log.Printf("netstat collect: %v", err)
		return
	}
	var rates []t.InterfaceRate
	if s.prevIface != nil {
		rates = netstat.CalculateRates(s.prevIface, currIface)
	}
	s.prevIface = currIface

	// 2. Ports + connections
	ports, _ := procnet.ScanListening()
	conns, _ := procnet.ScanConnections()
	ports = procnet.CountConnsPerPort(ports, conns)
	ports = s.procRes.AttributePorts(ports)

	// 3. Per-port byte rates from eBPF / fallback
	var topBytes []t.PortBytes
	s.mu.RLock()
	ps := s.portSamp
	s.mu.RUnlock()
	if ps != nil {
		topBytes, err = ps.CollectTop(50)
		if err != nil {
			s.log.Printf("port sampler: %v", err)
		}
	}

	// 4. Merge byte rates onto port records (by port number)
	byteMap := make(map[uint16]*t.PortBytes, len(topBytes))
	for i := range topBytes {
		byteMap[topBytes[i].Port] = &topBytes[i]
	}
	for i := range ports {
		if pb, ok := byteMap[ports[i].LocalPort]; ok {
			// PortBytes from eBPF/fallback is bytes/sec when sampler returns deltas.
			ports[i].RxBytesPerSec = float64(pb.RxBytes) / s.cfg.InterfaceInterval().Seconds()
			ports[i].TxBytesPerSec = float64(pb.TxBytes) / s.cfg.InterfaceInterval().Seconds()
			ports[i].TotalBps = ports[i].RxBytesPerSec + ports[i].TxBytesPerSec
		}
	}

	// 5. Compute totals
	var totalRx, totalTx float64
	for _, r := range rates {
		// Skip loopback
		if r.Name == "lo" {
			continue
		}
		totalRx += r.RxBytesPerSec
		totalTx += r.TxBytesPerSec
	}
	established := 0
	for _, c := range conns {
		if c.State == t.StateEstablished {
			established++
		}
	}

	// 6. Top ports by total bps
	topPorts := make([]t.Port, len(ports))
	copy(topPorts, ports)
	sort.Slice(topPorts, func(i, j int) bool {
		return topPorts[i].TotalBps > topPorts[j].TotalBps
	})
	if len(topPorts) > 20 {
		topPorts = topPorts[:20]
	}

	snap := t.Snapshot{
		TS:         now,
		Interfaces: currIface,
		Rates:      rates,
		Ports:      ports,
		TopPorts:   topPorts,
		Totals: t.SnapshotTotals{
			RxBytesPerSec:   totalRx,
			TxBytesPerSec:   totalTx,
			ListeningPorts:  len(ports),
			ActiveConns:     len(conns),
			EstablishedConn: established,
		},
	}

	// 7. Persist time-series points
	if s.mem != nil {
		s.mem.AppendTotals(t.HistoryPoint{
			TS:            now,
			RxBytesPerSec: totalRx,
			TxBytesPerSec: totalTx,
		})
		for _, r := range rates {
			s.mem.AppendInterface(r.Name, t.HistoryPoint{
				TS:            now,
				RxBytesPerSec: r.RxBytesPerSec,
				TxBytesPerSec: r.TxBytesPerSec,
			})
		}
		for _, p := range topPorts {
			s.mem.AppendPort(p.Protocol, p.LocalPort, t.HistoryPoint{
				TS:            now,
				RxBytesPerSec: p.RxBytesPerSec,
				TxBytesPerSec: p.TxBytesPerSec,
			})
		}
	}

	// 8. Publish
	s.mu.Lock()
	s.latest = snap
	subs := s.subscribers
	s.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- snap:
		default:
			// Drop on slow consumer.
		}
	}
}

// CollectConnections returns the latest connection list, GeoIP-enriched.
// Called from the HTTP layer (it's heavier than the regular tick).
func (s *Sampler) CollectConnections() ([]t.Connection, error) {
	conns, err := procnet.ScanConnections()
	if err != nil {
		return nil, err
	}
	conns = s.procRes.AttributeConnections(conns)
	if s.geo != nil {
		for i := range conns {
			country, asn := s.geo.Enrich(conns[i].RemoteIP)
			conns[i].Country = country
			conns[i].ASN = asn
		}
	}
	return conns, nil
}

func (s *Sampler) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.portSamp != nil {
		_ = s.portSamp.Close()
	}
	for _, ch := range s.subscribers {
		close(ch)
	}
	s.subscribers = nil
}

// TryEBPF attempts to start an eBPF sampler on the given interface. If
// it fails, returns a fallback sampler instead. Never returns nil.
func TryEBPF(iface string, log Logger) PerPortSampler {
	c, err := ebpf.New(iface)
	if err != nil {
		log.Printf("eBPF unavailable, using fallback sampler: %v", err)
		return fallback.New()
	}
	log.Printf("eBPF attached to %s", c.Iface())
	return c
}
