// Package db provides a persistent SQLite wrapper for queue state,
// favorites, and play history.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ytmgo/internal/queue"
	"ytmgo/internal/settings"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB with lifecycle management.
type DB struct {
	*sql.DB
}

// dbPath returns the path to the SQLite database file, creating the
// config directory if necessary.
func dbPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "ytmgo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create config dir %s: %w", dir, err)
	}
	return filepath.Join(dir, "ytmgo.db"), nil
}

// schema contains the DDL statements executed on database open.
const schema = `
CREATE TABLE IF NOT EXISTS queue_state (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    tracks      TEXT NOT NULL DEFAULT '[]',
    current_idx INTEGER NOT NULL DEFAULT -1,
    shuffle     INTEGER NOT NULL DEFAULT 0,
    repeat      INTEGER NOT NULL DEFAULT 0,
    repeat_all  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS favorites (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    track_id     TEXT NOT NULL UNIQUE,
    title        TEXT NOT NULL,
    artist       TEXT NOT NULL DEFAULT '',
    duration     TEXT NOT NULL DEFAULT '',
    duration_sec INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS play_history (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    track_id     TEXT NOT NULL,
    title        TEXT NOT NULL,
    artist       TEXT NOT NULL DEFAULT '',
    played_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS settings (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    playback_mode       INTEGER NOT NULL DEFAULT 0,
    default_volume      INTEGER NOT NULL DEFAULT 80,
    search_limit        INTEGER NOT NULL DEFAULT 20,
    download_dir        TEXT NOT NULL DEFAULT 'downloads',
    cookie_browser      TEXT NOT NULL DEFAULT 'brave',
    user_agent          TEXT NOT NULL DEFAULT '',
    show_quotes         INTEGER NOT NULL DEFAULT 1,
    discord_rpc_enabled INTEGER NOT NULL DEFAULT 1
);
`

const insertDefaultQueueState = `INSERT OR IGNORE INTO queue_state (id, tracks, current_idx, shuffle, repeat, repeat_all) VALUES (1, '[]', -1, 0, 0, 0);`
const insertDefaultSettings = `INSERT OR IGNORE INTO settings (id, playback_mode, default_volume, search_limit, download_dir, cookie_browser, user_agent, show_quotes, discord_rpc_enabled) VALUES (1, 0, 80, 20, 'downloads', 'brave', '', 1, 1);`

// Open opens (or creates) the SQLite database, runs migrations, and
// returns a DB handle. The database is opened with WAL journal mode,
// foreign keys enabled, and immediate transaction locking.
func Open() (*DB, error) {
	path, err := dbPath()
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", path, err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: create schema: %w", err)
	}

	// Migration: add discord_rpc_enabled column for databases created before
	// the Discord RPC feature was added. Must run before any INSERT that
	// references the column, or existing databases will fail with
	// "table settings has no column named discord_rpc_enabled".
	db.Exec(`ALTER TABLE settings ADD COLUMN discord_rpc_enabled INTEGER NOT NULL DEFAULT 1`)

	if _, err := db.Exec(insertDefaultQueueState); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: insert default queue state: %w", err)
	}

	if _, err := db.Exec(insertDefaultSettings); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: insert default settings: %w", err)
	}

	return &DB{db}, nil
}

// Close closes the underlying SQLite database connection.
func (d *DB) Close() error {
	return d.DB.Close()
}

// ─── Queue ──────────────────────────────────────────────────────────────

// LoadQueue reads the persisted queue state from the database.
// Returns tracks, shuffle flags, and any error. currentIndex is always set
// to -1 by the caller (Queue.LoadData) — see queue.go.
func (d *DB) LoadQueue() (tracks []queue.Track, shuffle, repeat, repeatAll bool, err error) {
	var rawTracks string
	var s, r, ra int // SQLite stores booleans as INTEGER 0/1
	row := d.QueryRow(`SELECT tracks, shuffle, repeat, repeat_all FROM queue_state WHERE id = 1`)
	if err := row.Scan(&rawTracks, &s, &r, &ra); err != nil {
		return nil, false, false, false, fmt.Errorf("load queue: %w", err)
	}
	shuffle, repeat, repeatAll = s != 0, r != 0, ra != 0
	if err := json.Unmarshal([]byte(rawTracks), &tracks); err != nil {
		return nil, false, false, false, fmt.Errorf("load queue: parse tracks: %w", err)
	}
	if tracks == nil {
		tracks = []queue.Track{}
	}
	return tracks, shuffle, repeat, repeatAll, nil
}

// SaveQueue writes the current queue state to the database.
func (d *DB) SaveQueue(tracks []queue.Track, currentIndex int, shuffle, repeat, repeatAll bool) error {
	raw, err := json.Marshal(tracks)
	if err != nil {
		return fmt.Errorf("save queue: encode tracks: %w", err)
	}
	_, err = d.Exec(
		`UPDATE queue_state SET tracks = ?, current_idx = ?, shuffle = ?, repeat = ?, repeat_all = ? WHERE id = 1`,
		string(raw), currentIndex, boolInt(shuffle), boolInt(repeat), boolInt(repeatAll),
	)
	if err != nil {
		return fmt.Errorf("save queue: %w", err)
	}
	return nil
}

// ─── Favorites ──────────────────────────────────────────────────────────

// LoadFavorites reads all favorited tracks from the database, ordered
// most-recent-first (descending by id).
func (d *DB) LoadFavorites() ([]queue.Track, error) {
	rows, err := d.Query(`SELECT track_id, title, artist, duration, duration_sec FROM favorites ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("load favorites: %w", err)
	}
	defer rows.Close()

	var favs []queue.Track
	for rows.Next() {
		var t queue.Track
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.Duration, &t.DurationSec); err != nil {
			return nil, fmt.Errorf("load favorites: scan: %w", err)
		}
		favs = append(favs, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load favorites: rows: %w", err)
	}
	if favs == nil {
		favs = []queue.Track{}
	}
	return favs, nil
}

// SaveFavorites replaces the entire favorites list with the provided tracks.
func (d *DB) SaveFavorites(tracks []queue.Track) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("save favorites: begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM favorites`); err != nil {
		return fmt.Errorf("save favorites: delete: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT INTO favorites (track_id, title, artist, duration, duration_sec) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("save favorites: prepare: %w", err)
	}
	defer stmt.Close()

	for _, t := range tracks {
		if _, err := stmt.Exec(t.ID, t.Title, t.Artist, t.Duration, t.DurationSec); err != nil {
			return fmt.Errorf("save favorites: insert %q: %w", t.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save favorites: commit: %w", err)
	}
	return nil
}

// ─── Play history ───────────────────────────────────────────────────────

// RecordPlay inserts a play history entry for the given track.
// The played_at timestamp is automatically set by SQLite.
func (d *DB) RecordPlay(t queue.Track) error {
	_, err := d.Exec(`INSERT INTO play_history (track_id, title, artist) VALUES (?, ?, ?)`,
		t.ID, t.Title, t.Artist)
	if err != nil {
		return fmt.Errorf("record play: %w", err)
	}
	return nil
}

// ─── Settings ────────────────────────────────────────────────────────────

// LoadSettings reads the single settings row from the database.
// Returns Defaults if the row doesn't exist or any error occurs.
func (d *DB) LoadSettings() (*settings.Settings, error) {
	var s settings.Settings
	var showQuotes, discordRPC int
	row := d.QueryRow(`SELECT playback_mode, default_volume, search_limit, download_dir, cookie_browser, user_agent, show_quotes, discord_rpc_enabled FROM settings WHERE id = 1`)
	if err := row.Scan(&s.PlaybackMode, &s.DefaultVolume, &s.SearchLimit, &s.DownloadDir, &s.CookieBrowser, &s.UserAgent, &showQuotes, &discordRPC); err != nil {
		return settings.Defaults(), fmt.Errorf("load settings: %w", err)
	}
	s.ShowQuotes = showQuotes != 0
	s.DiscordRPCEnabled = discordRPC != 0
	return &s, nil
}

// SaveSettings writes settings to the database.
func (d *DB) SaveSettings(s *settings.Settings) error {
	_, err := d.Exec(
		`UPDATE settings SET playback_mode = ?, default_volume = ?, search_limit = ?, download_dir = ?, cookie_browser = ?, user_agent = ?, show_quotes = ?, discord_rpc_enabled = ? WHERE id = 1`,
		s.PlaybackMode, s.DefaultVolume, s.SearchLimit, s.DownloadDir, s.CookieBrowser, s.UserAgent, boolInt(s.ShowQuotes), boolInt(s.DiscordRPCEnabled),
	)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

// ─── Helpers ────────────────────────────────────────────────────────────

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
