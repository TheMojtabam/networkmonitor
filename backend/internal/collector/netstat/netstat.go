// Package netstat parses /proc/net/dev for per-interface counters.
package netstat

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	t "github.com/mojtaba/portsleuth/backend/internal/collector"
)

// Collect reads /proc/net/dev and returns a snapshot of interface counters.
func Collect() ([]t.InterfaceStats, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var stats []t.InterfaceStats
	scanner := bufio.NewScanner(f)
	now := time.Now().UTC()
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= 2 {
			continue // header
		}
		line := strings.TrimSpace(scanner.Text())
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}
		name := strings.TrimSpace(line[:colonIdx])
		fields := strings.Fields(line[colonIdx+1:])
		if len(fields) < 16 {
			continue
		}
		stats = append(stats, t.InterfaceStats{
			Name:      name,
			RxBytes:   parseUint64(fields[0]),
			RxPackets: parseUint64(fields[1]),
			RxErrors:  parseUint64(fields[2]),
			RxDropped: parseUint64(fields[3]),
			TxBytes:   parseUint64(fields[8]),
			TxPackets: parseUint64(fields[9]),
			TxErrors:  parseUint64(fields[10]),
			TxDropped: parseUint64(fields[11]),
			TS:        now,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(stats) == 0 {
		return nil, errors.New("no network interfaces found")
	}
	return stats, nil
}

// CalculateRates computes per-second rates between two snapshots.
func CalculateRates(prev, curr []t.InterfaceStats) []t.InterfaceRate {
	prevMap := make(map[string]t.InterfaceStats, len(prev))
	for _, s := range prev {
		prevMap[s.Name] = s
	}
	out := make([]t.InterfaceRate, 0, len(curr))
	for _, c := range curr {
		p, ok := prevMap[c.Name]
		if !ok {
			continue
		}
		dt := c.TS.Sub(p.TS).Seconds()
		if dt <= 0 {
			continue
		}
		// Counters can wrap (rare on 64-bit); guard with a non-negative diff.
		out = append(out, t.InterfaceRate{
			Name:          c.Name,
			RxBytesPerSec: nonNegDiff(c.RxBytes, p.RxBytes) / dt,
			TxBytesPerSec: nonNegDiff(c.TxBytes, p.TxBytes) / dt,
			RxPktsPerSec:  nonNegDiff(c.RxPackets, p.RxPackets) / dt,
			TxPktsPerSec:  nonNegDiff(c.TxPackets, p.TxPackets) / dt,
			TS:            c.TS,
		})
	}
	return out
}

func nonNegDiff(a, b uint64) float64 {
	if a < b {
		return 0
	}
	return float64(a - b)
}

func parseUint64(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}
