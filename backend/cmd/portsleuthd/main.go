package main

import (
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/mojtaba/portsleuth/backend/internal/netstat"
	"github.com/mojtaba/portsleuth/backend/internal/portbytes"
	"github.com/mojtaba/portsleuth/backend/internal/sysinfo"
)

type envelope map[string]any

//go:embed web/*
var webFS embed.FS

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

	// Serve embedded frontend (SPA)
	if sub, err := fs.Sub(webFS, "web"); err == nil {
		fsHandler := http.FileServer(http.FS(sub))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Let API routes pass
			if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
				http.NotFound(w, r)
				return
			}
			// SPA fallback to index.html
			if r.URL.Path != "/" {
				if _, err := fs.Stat(sub, r.URL.Path[1:]); err != nil {
					r.URL.Path = "/"
				}
			}
			fsHandler.ServeHTTP(w, r)
		})
	}

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

	// Per-port bytes (eBPF)
	mux.HandleFunc("/api/net/port-bytes", func(w http.ResponseWriter, r *http.Request) {
		iface := r.URL.Query().Get("iface")
		if iface == "" {
			iface = "eth0"
		}
		limit := 50
		c, err := portbytes.New(iface)
		if err != nil {
			writeJSON(w, 501, envelope{
				"error":   err.Error(),
				"message": "per-port bytes needs eBPF + CAP_NET_ADMIN/CAP_BPF. Try running as root and ensure clang/llvm + bpf2go-generated objects are built.",
			})
			return
		}
		defer c.Close()
		rows, err := c.CollectTop(limit)
		if err != nil {
			writeJSON(w, 500, envelope{"error": err.Error()})
			return
		}
		writeJSON(w, 200, envelope{"iface": iface, "top": rows})
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
