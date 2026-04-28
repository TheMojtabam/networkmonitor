package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/mojtaba/portsleuth/backend/internal/netstat"
	"github.com/mojtaba/portsleuth/backend/internal/sysinfo"
)

type envelope map[string]any

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

var (
	// Global cache for interface rate calculations (1s minimum interval)
	ifaceCache = netstat.NewCache(1 * time.Second)
)

func main() {
	listen := flag.String("listen", ":1234", "listen address")
	flag.Parse()

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, envelope{"ok": true, "ts": time.Now().UTC()})
	})

	// System info
	mux.HandleFunc("/api/sys", func(w http.ResponseWriter, r *http.Request) {
		si, err := sysinfo.Collect()
		if err != nil {
			writeJSON(w, 500, envelope{"error": err.Error()})
			return
		}
		writeJSON(w, 200, si)
	})

	// Network interfaces: stats + rates
	mux.HandleFunc("/api/net/interfaces", func(w http.ResponseWriter, r *http.Request) {
		stats, rates, err := ifaceCache.GetInterfacesWithRates()
		if err != nil {
			writeJSON(w, 500, envelope{"error": err.Error()})
			return
		}

		writeJSON(w, 200, envelope{
			"interfaces": stats,
			"rates":      rates,
		})
	})

	// Listening ports with connection counts
	mux.HandleFunc("/api/net/ports", func(w http.ResponseWriter, r *http.Request) {
		ports, err := netstat.CollectListeningPorts()
		if err != nil {
			writeJSON(w, 500, envelope{"error": err.Error()})
			return
		}

		// Get active connections for counting
		conns, err := netstat.CollectConnections()
		if err != nil {
			log.Printf("Warning: failed to collect connections: %v", err)
		} else {
			ports = netstat.CountConnectionsPerPort(ports, conns)
		}

		writeJSON(w, 200, envelope{
			"ports": ports,
		})
	})

	// Active connections (detailed)
	mux.HandleFunc("/api/net/connections", func(w http.ResponseWriter, r *http.Request) {
		conns, err := netstat.CollectConnections()
		if err != nil {
			writeJSON(w, 500, envelope{"error": err.Error()})
			return
		}

		writeJSON(w, 200, envelope{
			"connections": conns,
		})
	})

	// Per-port bytes (eBPF or nftables)
	// NOTE: This requires elevated privileges and setup
	mux.HandleFunc("/api/net/port-bytes", func(w http.ResponseWriter, r *http.Request) {
		collector, err := netstat.NewPortBytesCollector()
		if err != nil {
			writeJSON(w, 500, envelope{
				"error":   err.Error(),
				"message": "Per-port byte accounting requires eBPF or nftables setup. See documentation.",
			})
			return
		}
		defer collector.Close()

		portBytes, err := collector.Collect()
		if err != nil {
			writeJSON(w, 500, envelope{"error": err.Error()})
			return
		}

		writeJSON(w, 200, envelope{
			"portBytes": portBytes,
		})
	})

	// SS fallback (alternative port listing using 'ss' command)
	mux.HandleFunc("/api/net/ports-ss", func(w http.ResponseWriter, r *http.Request) {
		ssCollector := netstat.NewSSCollector()
		ports, err := ssCollector.CollectWithSS()
		if err != nil {
			writeJSON(w, 500, envelope{"error": err.Error()})
			return
		}

		writeJSON(w, 200, envelope{
			"ports": ports,
		})
	})

	log.Printf("PortSleuth backend listening on %s", *listen)
	log.Printf("API endpoints:")
	log.Printf("  - /api/health")
	log.Printf("  - /api/sys")
	log.Printf("  - /api/net/interfaces")
	log.Printf("  - /api/net/ports")
	log.Printf("  - /api/net/connections")
	log.Printf("  - /api/net/port-bytes (requires setup)")
	log.Printf("  - /api/net/ports-ss (ss fallback)")

	if err := http.ListenAndServe(*listen, mux); err != nil {
		log.Fatal(err)
	}
}
