// Package store provides in-memory time-series storage with optional
// SQLite persistence for long-term aggregates.
//
// The hot path is an in-memory ring buffer keyed by series name. Each
// series holds a fixed-capacity slice of (timestamp, value) points. New
// points overwrite the oldest when the buffer is full.
package store

import (
	"sync"
	"time"

	t "github.com/mojtaba/portsleuth/backend/internal/collector"
)

// Series is a fixed-capacity ring buffer of HistoryPoints.
type Series struct {
	points []t.HistoryPoint
	head   int
	size   int
	cap    int
}

// NewSeries creates a series that holds up to capacity points.
func NewSeries(capacity int) *Series {
	return &Series{
		points: make([]t.HistoryPoint, capacity),
		cap:    capacity,
	}
}

// Append adds a new point, overwriting the oldest if full.
func (s *Series) Append(p t.HistoryPoint) {
	s.points[s.head] = p
	s.head = (s.head + 1) % s.cap
	if s.size < s.cap {
		s.size++
	}
}

// Snapshot returns all points in chronological order.
func (s *Series) Snapshot() []t.HistoryPoint {
	out := make([]t.HistoryPoint, 0, s.size)
	if s.size < s.cap {
		out = append(out, s.points[:s.size]...)
		return out
	}
	// Buffer is full: oldest is at head, newest at head-1.
	out = append(out, s.points[s.head:]...)
	out = append(out, s.points[:s.head]...)
	return out
}

// Since returns points with timestamp >= t.
func (s *Series) Since(after time.Time) []t.HistoryPoint {
	all := s.Snapshot()
	for i, p := range all {
		if p.TS.After(after) || p.TS.Equal(after) {
			return all[i:]
		}
	}
	return nil
}

// Memory is the top-level in-memory store. It holds an interface-total
// series plus a per-port series map. Safe for concurrent use.
type Memory struct {
	mu         sync.RWMutex
	totals     *Series
	perIface   map[string]*Series
	perPort    map[portKey]*Series
	cap        int
	windowSecs int
}

type portKey struct {
	port  uint16
	proto t.Protocol
}

// NewMemory creates a store sized for `windowHours` of `pointsPerSec` points.
func NewMemory(windowHours int, pointsPerSec int) *Memory {
	cap := windowHours * 3600 * pointsPerSec
	if cap < 60 {
		cap = 60
	}
	return &Memory{
		totals:     NewSeries(cap),
		perIface:   map[string]*Series{},
		perPort:    map[portKey]*Series{},
		cap:        cap,
		windowSecs: windowHours * 3600,
	}
}

// AppendTotals records a global RX/TX point.
func (m *Memory) AppendTotals(p t.HistoryPoint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totals.Append(p)
}

// AppendInterface records per-interface rates.
func (m *Memory) AppendInterface(name string, p t.HistoryPoint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.perIface[name]
	if !ok {
		s = NewSeries(m.cap)
		m.perIface[name] = s
	}
	s.Append(p)
}

// AppendPort records per-port rates.
func (m *Memory) AppendPort(proto t.Protocol, port uint16, p t.HistoryPoint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := portKey{port: port, proto: proto}
	s, ok := m.perPort[k]
	if !ok {
		s = NewSeries(m.cap)
		m.perPort[k] = s
	}
	s.Append(p)
}

// Totals returns the global series points after `t`.
func (m *Memory) Totals(after time.Time) []t.HistoryPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totals.Since(after)
}

// Interface returns per-interface points after `t`.
func (m *Memory) Interface(name string, after time.Time) []t.HistoryPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.perIface[name]; ok {
		return s.Since(after)
	}
	return nil
}

// Port returns per-port points after `t`.
func (m *Memory) Port(proto t.Protocol, port uint16, after time.Time) []t.HistoryPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.perPort[portKey{port: port, proto: proto}]; ok {
		return s.Since(after)
	}
	return nil
}
