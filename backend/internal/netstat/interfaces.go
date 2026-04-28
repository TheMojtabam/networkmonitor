package netstat

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// InterfaceStats represents network interface statistics
type InterfaceStats struct {
	Name      string    `json:"name"`
	RxBytes   uint64    `json:"rxBytes"`
	RxPackets uint64    `json:"rxPackets"`
	RxErrors  uint64    `json:"rxErrors"`
	RxDropped uint64    `json:"rxDropped"`
	TxBytes   uint64    `json:"txBytes"`
	TxPackets uint64    `json:"txPackets"`
	TxErrors  uint64    `json:"txErrors"`
	TxDropped uint64    `json:"txDropped"`
	TS        time.Time `json:"ts"`
}

// InterfaceRates represents calculated rates between two samples
type InterfaceRates struct {
	Name         string    `json:"name"`
	RxBytesPerSec float64  `json:"rxBytesPerSec"`
	TxBytesPerSec float64  `json:"txBytesPerSec"`
	RxPktsPerSec  float64  `json:"rxPktsPerSec"`
	TxPktsPerSec  float64  `json:"txPktsPerSec"`
	Interval      float64  `json:"interval"` // seconds
	TS            time.Time `json:"ts"`
}

// CollectInterfaces reads /proc/net/dev and returns current interface statistics
func CollectInterfaces() ([]InterfaceStats, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var stats []InterfaceStats
	scanner := bufio.NewScanner(f)
	lineNum := 0
	now := time.Now().UTC()

	for scanner.Scan() {
		lineNum++
		if lineNum <= 2 {
			// Skip header lines
			continue
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

		stat := InterfaceStats{
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
		}

		stats = append(stats, stat)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(stats) == 0 {
		return nil, errors.New("no network interfaces found")
	}

	return stats, nil
}

// CalculateRates computes per-second rates between two interface snapshots
func CalculateRates(prev, curr []InterfaceStats) []InterfaceRates {
	prevMap := make(map[string]InterfaceStats)
	for _, s := range prev {
		prevMap[s.Name] = s
	}

	var rates []InterfaceRates

	for _, cur := range curr {
		p, ok := prevMap[cur.Name]
		if !ok {
			// New interface appeared, skip rate calculation
			continue
		}

		interval := cur.TS.Sub(p.TS).Seconds()
		if interval <= 0 {
			continue
		}

		rate := InterfaceRates{
			Name:         cur.Name,
			RxBytesPerSec: float64(cur.RxBytes-p.RxBytes) / interval,
			TxBytesPerSec: float64(cur.TxBytes-p.TxBytes) / interval,
			RxPktsPerSec:  float64(cur.RxPackets-p.RxPackets) / interval,
			TxPktsPerSec:  float64(cur.TxPackets-p.TxPackets) / interval,
			Interval:     interval,
			TS:           cur.TS,
		}

		rates = append(rates, rate)
	}

	return rates
}

func parseUint64(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}
