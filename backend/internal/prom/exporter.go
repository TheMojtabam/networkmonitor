// Package prom exposes Prometheus metrics derived from the latest snapshot.
package prom

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	t "github.com/mojtaba/portsleuth/backend/internal/collector"
)

// Exporter holds gauges that mirror Snapshot fields.
type Exporter struct {
	mu       sync.Mutex
	registry *prometheus.Registry

	totalRx     prometheus.Gauge
	totalTx     prometheus.Gauge
	listenPorts prometheus.Gauge
	connEstab   prometheus.Gauge

	ifaceRxBytes *prometheus.GaugeVec
	ifaceTxBytes *prometheus.GaugeVec
	portRxBytes  *prometheus.GaugeVec
	portTxBytes  *prometheus.GaugeVec
	portConns    *prometheus.GaugeVec
}

// New constructs an exporter with registered gauges.
func New() *Exporter {
	r := prometheus.NewRegistry()
	e := &Exporter{
		registry: r,
		totalRx: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "portsleuth_rx_bytes_per_sec_total",
			Help: "Total received bytes/sec across all interfaces",
		}),
		totalTx: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "portsleuth_tx_bytes_per_sec_total",
			Help: "Total transmitted bytes/sec across all interfaces",
		}),
		listenPorts: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "portsleuth_listening_ports",
			Help: "Number of listening ports",
		}),
		connEstab: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "portsleuth_established_connections",
			Help: "Number of ESTABLISHED connections",
		}),
		ifaceRxBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "portsleuth_interface_rx_bytes_per_sec",
			Help: "Per-interface RX bytes/sec",
		}, []string{"interface"}),
		ifaceTxBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "portsleuth_interface_tx_bytes_per_sec",
			Help: "Per-interface TX bytes/sec",
		}, []string{"interface"}),
		portRxBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "portsleuth_port_rx_bytes_per_sec",
			Help: "Per-port RX bytes/sec (top ports)",
		}, []string{"port", "protocol", "process"}),
		portTxBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "portsleuth_port_tx_bytes_per_sec",
			Help: "Per-port TX bytes/sec (top ports)",
		}, []string{"port", "protocol", "process"}),
		portConns: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "portsleuth_port_connections",
			Help: "Active connection count per listening port",
		}, []string{"port", "protocol", "process"}),
	}
	r.MustRegister(
		e.totalRx, e.totalTx, e.listenPorts, e.connEstab,
		e.ifaceRxBytes, e.ifaceTxBytes,
		e.portRxBytes, e.portTxBytes, e.portConns,
	)
	return e
}

// Update refreshes gauges from a snapshot.
func (e *Exporter) Update(snap t.Snapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.totalRx.Set(snap.Totals.RxBytesPerSec)
	e.totalTx.Set(snap.Totals.TxBytesPerSec)
	e.listenPorts.Set(float64(snap.Totals.ListeningPorts))
	e.connEstab.Set(float64(snap.Totals.EstablishedConn))

	// Reset vec gauges (otherwise stale labels persist)
	e.ifaceRxBytes.Reset()
	e.ifaceTxBytes.Reset()
	e.portRxBytes.Reset()
	e.portTxBytes.Reset()
	e.portConns.Reset()

	for _, r := range snap.Rates {
		e.ifaceRxBytes.WithLabelValues(r.Name).Set(r.RxBytesPerSec)
		e.ifaceTxBytes.WithLabelValues(r.Name).Set(r.TxBytesPerSec)
	}
	for _, p := range snap.TopPorts {
		port := itoa(int(p.LocalPort))
		e.portRxBytes.WithLabelValues(port, string(p.Protocol), p.Process).Set(p.RxBytesPerSec)
		e.portTxBytes.WithLabelValues(port, string(p.Protocol), p.Process).Set(p.TxBytesPerSec)
		e.portConns.WithLabelValues(port, string(p.Protocol), p.Process).Set(float64(p.ConnectionCount))
	}
}

// Handler returns an http.Handler that serves /metrics.
func (e *Exporter) Handler() http.Handler {
	return promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
