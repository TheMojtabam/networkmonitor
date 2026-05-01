// SQLite persistence for long-term, low-resolution history.
//
// Schema is intentionally minimal — a single `samples` table with one row
// per (series, timestamp) pair. Series names are short strings like
// "totals.rx", "iface.eth0.rx", "port.tcp.443.rx".
//
// We aggregate live data into 1-minute buckets before writing, keeping
// disk write rate sane.
package store

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	t "github.com/mojtaba/portsleuth/backend/internal/collector"
)

// SQLite is a thin wrapper around a sqlite database.
type SQLite struct {
	mu sync.Mutex
	db *sql.DB
}

// OpenSQLite opens (or creates) the given path and runs migrations.
// Returns nil DB if path is empty.
func OpenSQLite(path string) (*SQLite, error) {
	if path == "" {
		return nil, nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // sqlite is single-writer
	s := &SQLite{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *SQLite) migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS samples (
		series TEXT NOT NULL,
		ts     INTEGER NOT NULL,
		value  REAL NOT NULL,
		PRIMARY KEY (series, ts)
	);
	CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples(ts);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Insert writes a single (series, ts, value) row.
func (s *SQLite) Insert(series string, ts time.Time, value float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO samples(series, ts, value) VALUES (?, ?, ?)`,
		series, ts.Unix(), value,
	)
	return err
}

// InsertBatch writes many rows in a single transaction.
func (s *SQLite) InsertBatch(rows []SQLRow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO samples(series, ts, value) VALUES (?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.Series, r.TS.Unix(), r.Value); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// SQLRow is a single sample to be persisted.
type SQLRow struct {
	Series string
	TS     time.Time
	Value  float64
}

// Query returns samples for `series` between `from` and `to`.
func (s *SQLite) Query(series string, from, to time.Time) ([]t.HistoryPoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(
		`SELECT ts, value FROM samples WHERE series = ? AND ts >= ? AND ts <= ? ORDER BY ts ASC`,
		series, from.Unix(), to.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []t.HistoryPoint
	for rows.Next() {
		var ts int64
		var v float64
		if err := rows.Scan(&ts, &v); err != nil {
			return nil, err
		}
		out = append(out, t.HistoryPoint{
			TS:            time.Unix(ts, 0).UTC(),
			RxBytesPerSec: v,
		})
	}
	return out, rows.Err()
}

// Prune removes samples older than retention.
func (s *SQLite) Prune(retention time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-retention).Unix()
	_, err := s.db.Exec(`DELETE FROM samples WHERE ts < ?`, cutoff)
	return err
}

// Close shuts down the database.
func (s *SQLite) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
