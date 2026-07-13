package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store is a thin SQLite wrapper for settings + saved test runs.
type Store struct {
	db   *sql.DB
	path string
}

// TestRun is one saved speed/bandwidth snapshot.
type TestRun struct {
	ID            int64
	Name          string
	Kind          string // "speed" | "bandwidth"
	DownloadMbps  float64
	UploadMbps    float64
	PingMs        float64
	DownloadPeak  float64
	UploadPeak    float64
	Server        string
	CreatedAt     time.Time
}

// Open creates (if needed) and opens riptide.db under the user config dir.
// Path: <UserConfigDir>/riptide/riptide.db
func Open() (*Store, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	path := filepath.Join(dir, "riptide.db")
	return OpenPath(path)
}

// OpenPath opens a specific database file (used by tests and Open).
func OpenPath(path string) (*Store, error) {
	d, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	d.SetMaxOpenConns(1)
	s := &Store{db: d, path: path}
	if err := s.migrate(); err != nil {
		_ = d.Close()
		return nil, err
	}
	return s, nil
}

func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("config dir: %v / home: %v", err, herr)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "riptide"), nil
}

// Path returns the on-disk database path.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY NOT NULL,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS test_runs (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	name           TEXT    NOT NULL,
	kind           TEXT    NOT NULL,
	download_mbps  REAL    NOT NULL DEFAULT 0,
	upload_mbps    REAL    NOT NULL DEFAULT 0,
	ping_ms        REAL    NOT NULL DEFAULT 0,
	download_peak  REAL    NOT NULL DEFAULT 0,
	upload_peak    REAL    NOT NULL DEFAULT 0,
	server         TEXT    NOT NULL DEFAULT '',
	created_at     TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_test_runs_created ON test_runs(created_at DESC);
`)
	return err
}

// GetSetting reads a setting value; missing keys return def.
func (s *Store) GetSetting(key, def string) string {
	if s == nil {
		return def
	}
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

// SetSetting writes a setting value.
func (s *Store) SetSetting(key, value string) error {
	if s == nil {
		return nil
	}
	_, err := s.db.Exec(`
INSERT INTO settings(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value
`, key, value)
	return err
}

// SaveTestRun inserts a named run and returns its id.
func (s *Store) SaveTestRun(r TestRun) (int64, error) {
	if s == nil {
		return 0, fmt.Errorf("database not open")
	}
	if r.Name == "" {
		r.Name = "Untitled"
	}
	if r.Kind == "" {
		r.Kind = "speed"
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	res, err := s.db.Exec(`
INSERT INTO test_runs(
	name, kind, download_mbps, upload_mbps, ping_ms,
	download_peak, upload_peak, server, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`, r.Name, r.Kind, r.DownloadMbps, r.UploadMbps, r.PingMs,
		r.DownloadPeak, r.UploadPeak, r.Server, r.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// LatestRuns returns the most recent n test runs (newest first).
func (s *Store) LatestRuns(n int) ([]TestRun, error) {
	if s == nil {
		return nil, nil
	}
	if n <= 0 {
		n = 10
	}
	rows, err := s.db.Query(`
SELECT id, name, kind, download_mbps, upload_mbps, ping_ms,
       download_peak, upload_peak, server, created_at
FROM test_runs
ORDER BY datetime(created_at) DESC, id DESC
LIMIT ?
`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TestRun
	for rows.Next() {
		var r TestRun
		var created string
		if err := rows.Scan(
			&r.ID, &r.Name, &r.Kind, &r.DownloadMbps, &r.UploadMbps, &r.PingMs,
			&r.DownloadPeak, &r.UploadPeak, &r.Server, &created,
		); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, created)
		if r.CreatedAt.IsZero() {
			// Best-effort for any older formats.
			r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountRuns returns total saved runs.
func (s *Store) CountRuns() (int, error) {
	if s == nil {
		return 0, nil
	}
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM test_runs`).Scan(&n)
	return n, err
}

// Averages holds the mean download/upload/ping across saved runs of a kind.
type Averages struct {
	Count        int
	DownloadMbps float64
	UploadMbps   float64
	PingMs       float64
}

// Averages returns the mean download, upload, and ping over all saved runs of
// the given kind (empty kind defaults to "speed"). Zero rows yields a Count of 0.
func (s *Store) Averages(kind string) (Averages, error) {
	if s == nil {
		return Averages{}, nil
	}
	if kind == "" {
		kind = "speed"
	}
	var a Averages
	err := s.db.QueryRow(`
SELECT COUNT(*),
       COALESCE(AVG(download_mbps), 0),
       COALESCE(AVG(upload_mbps), 0),
       COALESCE(AVG(ping_ms), 0)
FROM test_runs
WHERE kind = ?
`, kind).Scan(&a.Count, &a.DownloadMbps, &a.UploadMbps, &a.PingMs)
	if err != nil {
		return Averages{}, err
	}
	return a, nil
}

// Reset clears all test runs and optional settings keys (theme is kept unless wipeSettings).
func (s *Store) Reset(wipeSettings bool) error {
	if s == nil {
		return fmt.Errorf("database not open")
	}
	if _, err := s.db.Exec(`DELETE FROM test_runs`); err != nil {
		return err
	}
	if wipeSettings {
		if _, err := s.db.Exec(`DELETE FROM settings`); err != nil {
			return err
		}
	}
	// Best-effort reclaim; VACUUM is optional.
	_, _ = s.db.Exec(`VACUUM`)
	return nil
}

// FormatWhen renders a short local date/time for history rows.
func FormatWhen(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	local := t.Local()
	now := time.Now()
	if local.Year() == now.Year() && local.YearDay() == now.YearDay() {
		return local.Format("15:04")
	}
	if local.Year() == now.Year() {
		return local.Format("Jan 02 15:04")
	}
	return local.Format("2006-01-02 15:04")
}

// AutoName builds a default run name from kind + timestamp.
func AutoName(kind string, t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	label := "Speed Test"
	if strings.EqualFold(kind, "bandwidth") {
		label = "Bandwidth"
	}
	return fmt.Sprintf("%s · %s", label, t.Local().Format("Jan 02 15:04"))
}
