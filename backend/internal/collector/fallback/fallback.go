// Package fallback provides a non-eBPF per-port byte estimator using
// the `ss` command (iproute2) which reads kernel TCP_INFO. This is less
// accurate than eBPF (UDP isn't covered, byte counts come from TCP_INFO
// not packet inspection) but works without any special privileges.
package fallback

import (
	"bufio"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	t "github.com/mojtaba/portsleuth/backend/internal/collector"
)

// Sampler tracks delta of per-port byte counters between two `ss` snapshots.
type Sampler struct {
	mu       sync.Mutex
	prevSnap map[uint16]*portTotals
	prevTime time.Time
}

type portTotals struct {
	rxBytes uint64
	txBytes uint64
}

// New returns a fresh Sampler.
func New() *Sampler {
	return &Sampler{prevSnap: map[uint16]*portTotals{}}
}

// CollectTop returns top-N ports by RX+TX bytes per second.
// First call returns zero rates (no baseline yet); subsequent calls return rates.
func (s *Sampler) CollectTop(limit int) ([]t.PortBytes, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	curr, err := snapshot()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	dt := now.Sub(s.prevTime).Seconds()

	out := make([]t.PortBytes, 0, len(curr))
	if !s.prevTime.IsZero() && dt > 0 {
		for port, c := range curr {
			p := s.prevSnap[port]
			rxRate, txRate := uint64(0), uint64(0)
			if p != nil {
				rxRate = nonNegDiff(c.rxBytes, p.rxBytes)
				txRate = nonNegDiff(c.txBytes, p.txBytes)
			}
			out = append(out, t.PortBytes{
				Port:     port,
				Protocol: "tcp",
				RxBytes:  rxRate,
				TxBytes:  txRate,
			})
		}
	}

	s.prevSnap = curr
	s.prevTime = now

	// sort + cap
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].RxBytes+out[j].TxBytes > out[i].RxBytes+out[i].TxBytes {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Close is a no-op (here to match the eBPF Collector interface).
func (s *Sampler) Close() error { return nil }

// snapshot runs `ss -tin` and aggregates bytes_acked / bytes_received
// per local port. Output looks like:
//
//	State Recv-Q Send-Q Local Address:Port Peer Address:Port Process
//	ESTAB 0      0      127.0.0.1:5432     127.0.0.1:48210
//		 cubic wscale:7,7 rto:204 rtt:0.054/0.027 ato:40 mss:32768 ...
//		 bytes_sent:842 bytes_acked:842 bytes_received:1024 segs_out:5 ...
//
// We pair the address line with the metrics on the next indented line.
var (
	rxRe = regexp.MustCompile(`bytes_received:(\d+)`)
	txRe = regexp.MustCompile(`bytes_acked:(\d+)`)
)

func snapshot() (map[uint16]*portTotals, error) {
	cmd := exec.Command("ss", "-tin")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	totals := map[uint16]*portTotals{}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 1<<16), 1<<20)

	var lastPort uint16
	for scanner.Scan() {
		line := scanner.Text()
		// Address line: not indented, contains ":port"
		if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			lastPort = portFromAddr(fields[3])
			continue
		}
		// Indented metric line
		if lastPort == 0 {
			continue
		}
		entry, ok := totals[lastPort]
		if !ok {
			entry = &portTotals{}
			totals[lastPort] = entry
		}
		if m := rxRe.FindStringSubmatch(line); len(m) == 2 {
			v, _ := strconv.ParseUint(m[1], 10, 64)
			entry.rxBytes += v
		}
		if m := txRe.FindStringSubmatch(line); len(m) == 2 {
			v, _ := strconv.ParseUint(m[1], 10, 64)
			entry.txBytes += v
		}
	}
	return totals, scanner.Err()
}

func portFromAddr(addr string) uint16 {
	// "10.0.0.5:443" or "[::]:443"
	idx := strings.LastIndex(addr, ":")
	if idx == -1 {
		return 0
	}
	v, _ := strconv.ParseUint(addr[idx+1:], 10, 16)
	return uint16(v)
}

func nonNegDiff(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}
