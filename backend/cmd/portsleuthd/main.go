package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/mojtaba/portsleuth/backend/internal/sysinfo"
)

type envelope map[string]any

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	listen := flag.String("listen", ":1234", "listen address")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, envelope{"ok": true, "ts": time.Now().UTC()})
	})

	mux.HandleFunc("/api/sys", func(w http.ResponseWriter, r *http.Request) {
		si, err := sysinfo.Collect()
		if err != nil {
			writeJSON(w, 500, envelope{"error": err.Error()})
			return
		}
		writeJSON(w, 200, si)
	})

	// TODO: /api/net/interfaces
	// TODO: /api/net/ports (eBPF per-port bytes; fallback: ss/conntrack)

	log.Printf("PortSleuth backend listening on %s", *listen)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		log.Fatal(err)
	}
}
