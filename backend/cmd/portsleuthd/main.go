// Command portsleuthd is the PortSleuth backend daemon.
//
// It loads config, starts the sampler pipeline, attaches the eBPF
// (or fallback) per-port collector, runs the alert engine and the
// WebSocket pump, and serves the REST + SPA on the configured port.
//
// Graceful shutdown: SIGINT/SIGTERM stops the sampler, drains the
// WebSocket hub, flushes SQLite, and then exits.
package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mojtaba/portsleuth/backend/internal/alert"
	"github.com/mojtaba/portsleuth/backend/internal/auth"
	t "github.com/mojtaba/portsleuth/backend/internal/collector"
	"github.com/mojtaba/portsleuth/backend/internal/collector/sampler"
	"github.com/mojtaba/portsleuth/backend/internal/config"
	"github.com/mojtaba/portsleuth/backend/internal/geoip"
	"github.com/mojtaba/portsleuth/backend/internal/prom"
	"github.com/mojtaba/portsleuth/backend/internal/server"
	"github.com/mojtaba/portsleuth/backend/internal/store"
	"github.com/mojtaba/portsleuth/backend/internal/ws"
)

// Embedded SPA assets. The Vite build outputs to backend/cmd/portsleuthd/web
// before `go build`. Deployment notes are in README.md.
//
//go:embed all:web
var webFS embed.FS

func main() {
	configPath := flag.String("config", "", "path to config.yaml")
	listenOverride := flag.String("listen", "", "override server.listen (e.g. :1234)")
	flag.Parse()

	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)

	// 1. Load config.
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatalf("config: %v", err)
	}
	if *listenOverride != "" {
		cfg.Server.Listen = *listenOverride
	}
	logger.Printf("PortSleuth starting (listen=%s, ebpf=%v, sqlite=%q, auth=%v)",
		cfg.Server.Listen, cfg.Collector.EBPFEnabled, cfg.Storage.SQLitePath, cfg.Auth.Enabled)

	// 2. Storage.
	mem := store.NewMemory(cfg.Storage.MemoryWindowHours, 1)

	var sqliteStore *store.SQLite
	if cfg.Storage.SQLitePath != "" {
		sqliteStore, err = store.OpenSQLite(cfg.Storage.SQLitePath)
		if err != nil {
			logger.Printf("warning: sqlite unavailable: %v", err)
		} else {
			logger.Printf("sqlite history: %s", cfg.Storage.SQLitePath)
		}
	}

	// 3. GeoIP (optional).
	geoEnricher, err := geoip.Open(cfg.GeoIP)
	if err != nil {
		logger.Printf("warning: geoip unavailable: %v", err)
	}
	defer func() {
		if geoEnricher != nil {
			_ = geoEnricher.Close()
		}
	}()

	// 4. Auth.
	authMgr, err := auth.New(cfg.Auth)
	if err != nil {
		logger.Fatalf("auth: %v", err)
	}

	// 5. Alert engine.
	var alertEngine *alert.Engine
	if cfg.Alerts.Enabled {
		alertEngine, err = alert.NewEngine(cfg.Alerts.RulesFile, logger)
		if err != nil {
			logger.Printf("warning: alert engine: %v", err)
		}
	}

	// 6. Prometheus exporter (optional).
	var promExp *prom.Exporter
	if cfg.Prometheus.Enabled {
		promExp = prom.New()
	}

	// 7. WebSocket hub.
	wsHub := ws.NewHub(logger, nil)

	// 8. Sampler.
	samp := sampler.New(cfg.Collector, mem, geoEnricher, logger)

	// 9. Try to attach eBPF (with fallback).
	if cfg.Collector.EBPFEnabled {
		iface := pickInterface(cfg.Collector.EBPFInterfaces)
		ps := sampler.TryEBPF(iface, logger)
		samp.SetPortSampler(ps)
		defer func() {
			_ = ps.Close()
		}()
	}

	// 10. Wire snapshot subscribers BEFORE starting the sampler so the
	//     first snapshot is delivered to everyone.
	wsCh := samp.Subscribe()
	go wsHub.Pump(wsCh)

	if alertEngine != nil {
		alertCh := samp.Subscribe()
		go func() {
			for snap := range alertCh {
				events := alertEngine.Evaluate(snap)
				for _, ev := range events {
					wsHub.SendEvent("alert", ev)
				}
			}
		}()
	}

	if promExp != nil {
		promCh := samp.Subscribe()
		go func() {
			for snap := range promCh {
				promExp.Update(snap)
			}
		}()
	}

	if sqliteStore != nil {
		sqliteCh := samp.Subscribe()
		go runSQLiteAggregator(sqliteStore, sqliteCh, cfg.Storage.RetentionDays, logger)
		defer sqliteStore.Close()
	}

	// 11. Sampler context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go samp.Run(ctx)

	// 12. Build HTTP server.
	staticFS, err := spaFS()
	if err != nil {
		logger.Printf("warning: SPA assets not embedded: %v", err)
	}
	handler := server.Build(server.Deps{
		Cfg:      cfg,
		Sampler:  samp,
		Memory:   mem,
		Alerts:   alertEngine,
		Auth:     authMgr,
		WS:       wsHub,
		Prom:     promExp,
		StaticFS: staticFS,
		Log:      logger,
	})

	srv := &http.Server{
		Addr:         cfg.Server.Listen,
		Handler:      handler,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: 0, // no write timeout — WebSocket needs long-lived connections
		IdleTimeout:  120 * time.Second,
	}

	// 13. Run + graceful shutdown.
	go func() {
		logger.Printf("listening on %s", cfg.Server.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Printf("shutting down...")

	cancel() // stop sampler
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	logger.Printf("bye.")
}

// runSQLiteAggregator buckets snapshots into 1-minute averages and
// flushes batches to disk. Also prunes old rows once an hour.
func runSQLiteAggregator(s *store.SQLite, ch <-chan t.Snapshot, retentionDays int, log *log.Logger) {
	pruneTicker := time.NewTicker(time.Hour)
	defer pruneTicker.Stop()
	flushTicker := time.NewTicker(time.Minute)
	defer flushTicker.Stop()

	type bucket struct {
		count int
		rxSum float64
		txSum float64
		ts    time.Time
	}
	buckets := map[string]*bucket{}

	addSample := func(series string, ts time.Time, rx, tx float64) {
		b, ok := buckets[series]
		if !ok {
			b = &bucket{ts: ts}
			buckets[series] = b
		}
		b.count++
		b.rxSum += rx
		b.txSum += tx
	}

	for {
		select {
		case snap, ok := <-ch:
			if !ok {
				return
			}
			addSample("totals", snap.TS, snap.Totals.RxBytesPerSec, snap.Totals.TxBytesPerSec)
			for _, r := range snap.Rates {
				addSample("iface."+r.Name, snap.TS, r.RxBytesPerSec, r.TxBytesPerSec)
			}
		case <-flushTicker.C:
			if len(buckets) == 0 {
				continue
			}
			var rows []store.SQLRow
			for series, b := range buckets {
				if b.count == 0 {
					continue
				}
				rows = append(rows,
					store.SQLRow{Series: series + ".rx", TS: b.ts, Value: b.rxSum / float64(b.count)},
					store.SQLRow{Series: series + ".tx", TS: b.ts, Value: b.txSum / float64(b.count)},
				)
			}
			if err := s.InsertBatch(rows); err != nil {
				log.Printf("sqlite insert: %v", err)
			}
			buckets = map[string]*bucket{}
		case <-pruneTicker.C:
			if retentionDays > 0 {
				if err := s.Prune(time.Duration(retentionDays) * 24 * time.Hour); err != nil {
					log.Printf("sqlite prune: %v", err)
				}
			}
		}
	}
}

// pickInterface returns the first user-specified interface, or "auto" if "auto" is in the list.
func pickInterface(list []string) string {
	for _, name := range list {
		if name != "" {
			return name
		}
	}
	return "auto"
}

// spaFS returns the embedded SPA assets as an http.FileSystem.
// Returns nil (and no error) if the `web` directory is empty
// — useful in dev mode where the frontend is served by `vite dev`.
func spaFS() (http.FileSystem, error) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	// Probe: is there an index.html?
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, nil
	}
	return http.FS(sub), nil
}
