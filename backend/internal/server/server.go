// Package server wires the HTTP layer: REST endpoints, WebSocket, auth,
// metrics. It depends on the sampler, alert engine, and stores but
// doesn't own their lifecycle — that's the caller's job.
package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mojtaba/portsleuth/backend/internal/alert"
	"github.com/mojtaba/portsleuth/backend/internal/auth"
	t "github.com/mojtaba/portsleuth/backend/internal/collector"
	"github.com/mojtaba/portsleuth/backend/internal/collector/sampler"
	"github.com/mojtaba/portsleuth/backend/internal/config"
	"github.com/mojtaba/portsleuth/backend/internal/prom"
	"github.com/mojtaba/portsleuth/backend/internal/store"
	"github.com/mojtaba/portsleuth/backend/internal/ws"
)

// Logger is the local logger interface.
type Logger interface {
	Printf(format string, args ...any)
}

// Deps is everything the HTTP server needs.
type Deps struct {
	Cfg      *config.Config
	Sampler  *sampler.Sampler
	Memory   *store.Memory
	Alerts   *alert.Engine
	Auth     *auth.Manager
	WS       *ws.Hub
	Prom     *prom.Exporter
	StaticFS http.FileSystem
	Log      Logger
}

// Build assembles the http.Handler.
func Build(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "ts": time.Now().UTC()})
	})

	// ---- auth ----
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !d.Auth.Enabled() {
			writeJSON(w, 200, map[string]any{"token": "", "authDisabled": true})
			return
		}
		token, err := d.Auth.Login(body.Username, body.Password)
		if err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		writeJSON(w, 200, map[string]any{"token": token})
	})

	mux.HandleFunc("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromCtx(r.Context())
		if u == nil {
			writeJSON(w, 200, map[string]any{"authDisabled": !d.Auth.Enabled()})
			return
		}
		writeJSON(w, 200, map[string]any{"user": u})
	})

	// ---- snapshot (point-in-time) ----
	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, d.Sampler.Latest())
	})

	// ---- interfaces ----
	mux.HandleFunc("/api/net/interfaces", func(w http.ResponseWriter, r *http.Request) {
		snap := d.Sampler.Latest()
		writeJSON(w, 200, map[string]any{
			"interfaces": snap.Interfaces,
			"rates":      snap.Rates,
		})
	})

	// ---- ports ----
	mux.HandleFunc("/api/net/ports", func(w http.ResponseWriter, r *http.Request) {
		snap := d.Sampler.Latest()
		writeJSON(w, 200, map[string]any{"ports": snap.Ports})
	})

	// ---- top ports by bandwidth ----
	mux.HandleFunc("/api/net/top", func(w http.ResponseWriter, r *http.Request) {
		by := r.URL.Query().Get("by") // "bandwidth" | "rx" | "tx" | "connections"
		if by == "" {
			by = "bandwidth"
		}
		limit := atoi(r.URL.Query().Get("limit"), 20)
		snap := d.Sampler.Latest()
		ports := append([]t.Port(nil), snap.Ports...)
		switch by {
		case "rx":
			sortPorts(ports, func(a, b t.Port) bool { return a.RxBytesPerSec > b.RxBytesPerSec })
		case "tx":
			sortPorts(ports, func(a, b t.Port) bool { return a.TxBytesPerSec > b.TxBytesPerSec })
		case "connections":
			sortPorts(ports, func(a, b t.Port) bool { return a.ConnectionCount > b.ConnectionCount })
		default:
			sortPorts(ports, func(a, b t.Port) bool { return a.TotalBps > b.TotalBps })
		}
		if len(ports) > limit {
			ports = ports[:limit]
		}
		writeJSON(w, 200, map[string]any{"by": by, "limit": limit, "ports": ports})
	})

	// ---- connections ----
	mux.HandleFunc("/api/net/connections", func(w http.ResponseWriter, r *http.Request) {
		conns, err := d.Sampler.CollectConnections()
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"connections": conns})
	})

	// ---- history ----
	mux.HandleFunc("/api/history/totals", func(w http.ResponseWriter, r *http.Request) {
		after := parseAfter(r.URL.Query().Get("since"))
		if d.Memory == nil {
			writeJSON(w, 200, map[string]any{"points": []any{}})
			return
		}
		points := d.Memory.Totals(after)
		writeJSON(w, 200, map[string]any{"points": points})
	})

	mux.HandleFunc("/api/history/interface", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		after := parseAfter(r.URL.Query().Get("since"))
		if d.Memory == nil || name == "" {
			writeJSON(w, 400, map[string]any{"error": "missing name"})
			return
		}
		writeJSON(w, 200, map[string]any{"points": d.Memory.Interface(name, after)})
	})

	mux.HandleFunc("/api/history/port", func(w http.ResponseWriter, r *http.Request) {
		proto := t.Protocol(strings.ToLower(r.URL.Query().Get("protocol")))
		port := uint16(atoi(r.URL.Query().Get("port"), 0))
		after := parseAfter(r.URL.Query().Get("since"))
		if d.Memory == nil || port == 0 {
			writeJSON(w, 400, map[string]any{"error": "missing port"})
			return
		}
		if proto == "" {
			proto = t.ProtoTCP
		}
		writeJSON(w, 200, map[string]any{"points": d.Memory.Port(proto, port, after)})
	})

	// ---- alerts ----
	mux.HandleFunc("/api/alerts/rules", func(w http.ResponseWriter, r *http.Request) {
		if d.Alerts == nil {
			writeJSON(w, 200, map[string]any{"rules": []any{}})
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]any{"rules": d.Alerts.Rules()})
		case http.MethodPut:
			var body struct {
				Rules []alert.Rule `json:"rules"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", 400)
				return
			}
			if err := d.Alerts.SetRules(r.Context(), body.Rules); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			writeJSON(w, 200, map[string]any{"ok": true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/alerts/channels", func(w http.ResponseWriter, r *http.Request) {
		if d.Alerts == nil {
			writeJSON(w, 200, map[string]any{"channels": []any{}})
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]any{"channels": d.Alerts.Channels()})
		case http.MethodPut:
			var body struct {
				Channels []alert.Channel `json:"channels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", 400)
				return
			}
			if err := d.Alerts.SetChannels(body.Channels); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			writeJSON(w, 200, map[string]any{"ok": true})
		}
	})

	mux.HandleFunc("/api/alerts/events", func(w http.ResponseWriter, r *http.Request) {
		if d.Alerts == nil {
			writeJSON(w, 200, map[string]any{"events": []any{}})
			return
		}
		writeJSON(w, 200, map[string]any{"events": d.Alerts.RecentEvents()})
	})

	// ---- websocket ----
	if d.WS != nil {
		mux.Handle("/ws", d.WS.Handler())
	}

	// ---- prometheus ----
	if d.Prom != nil && d.Cfg.Prometheus.Enabled {
		mux.Handle(d.Cfg.Prometheus.Path, d.Prom.Handler())
	}

	// ---- static (frontend SPA) ----
	if d.StaticFS != nil {
		mux.Handle("/", spaHandler(d.StaticFS))
	}

	// Wrap entire mux with auth + CORS.
	var handler http.Handler = mux
	handler = withCORS(handler)
	if d.Auth != nil {
		handler = d.Auth.Middleware(handler)
	}
	return handler
}

// ----------------- helpers -----------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func atoi(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseAfter(s string) time.Time {
	if s == "" {
		return time.Now().Add(-1 * time.Hour)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if secs, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(secs, 0)
	}
	return time.Now().Add(-1 * time.Hour)
}

// sortPorts sorts in-place using a custom less function (avoids importing sort here).
func sortPorts(ports []t.Port, less func(a, b t.Port) bool) {
	// simple insertion sort — port slice is small (<200 typically)
	for i := 1; i < len(ports); i++ {
		j := i
		for j > 0 && less(ports[j], ports[j-1]) {
			ports[j], ports[j-1] = ports[j-1], ports[j]
			j--
		}
	}
}

// withCORS adds permissive CORS headers (frontend dev typically runs on a different port).
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spaHandler serves a single-page app: any path that doesn't exist falls back to /index.html.
func spaHandler(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API paths fall through (they're registered with explicit handlers above).
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" || r.URL.Path == "/metrics" {
			http.NotFound(w, r)
			return
		}
		f, err := fsys.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			r.URL.Path = "/"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(w, r)
	})
}
