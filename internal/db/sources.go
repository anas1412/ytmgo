package db

import "fmt"

// Source is an album or a playlist something was played from, for the
// GUI's quick tiles: what you listened to lately, newest first.
type Source struct {
	Kind     string `json:"kind"` // "album" or "playlist"
	ID       string `json:"id"`   // MPREb_… or VL…
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	CoverURL string `json:"coverURL"`
}

// sourcesKept is how many sources are remembered; the oldest go first.
const sourcesKept = 50

// RecordSource notes that s was played from now. Playing it again moves
// it to the front, with what it shows brought up to date.
func (d *DB) RecordSource(s Source) error {
	if s.Kind == "" || s.ID == "" {
		return fmt.Errorf("record source: no kind or id")
	}
	_, err := d.Exec(`INSERT INTO recent_sources (kind, id, title, subtitle, cover_url, played_at)
		VALUES (?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		ON CONFLICT (kind, id) DO UPDATE SET title = excluded.title, subtitle = excluded.subtitle,
			cover_url = excluded.cover_url, played_at = excluded.played_at`,
		s.Kind, s.ID, s.Title, s.Subtitle, s.CoverURL)
	if err != nil {
		return fmt.Errorf("record source: %w", err)
	}
	_, err = d.Exec(`DELETE FROM recent_sources WHERE rowid NOT IN
		(SELECT rowid FROM recent_sources ORDER BY played_at DESC LIMIT ?)`, sourcesKept)
	if err != nil {
		return fmt.Errorf("record source: trim: %w", err)
	}
	return nil
}

// LoadSources lists the newest limit sources, newest first.
func (d *DB) LoadSources(limit int) ([]Source, error) {
	rows, err := d.Query(`SELECT kind, id, title, subtitle, cover_url FROM recent_sources
		ORDER BY played_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("load sources: %w", err)
	}
	defer rows.Close()
	out := []Source{}
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.Kind, &s.ID, &s.Title, &s.Subtitle, &s.CoverURL); err != nil {
			return nil, fmt.Errorf("load sources: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
