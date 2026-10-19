package db

import "fmt"

// Saved is an artist followed or an album liked, kept for the GUI's
// Favourites: Kind "artist" with a UC… id, or "album" with an MPREb_…
// one; Subtitle is an album's artist.
type Saved struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	CoverURL string `json:"coverURL"`
}

// Save keeps s, or brings what it shows up to date if it is kept already.
func (d *DB) Save(s Saved) error {
	if s.Kind == "" || s.ID == "" {
		return fmt.Errorf("save: no kind or id")
	}
	_, err := d.Exec(`INSERT INTO saved_items (kind, id, title, subtitle, cover_url) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (kind, id) DO UPDATE SET title = excluded.title, subtitle = excluded.subtitle, cover_url = excluded.cover_url`,
		s.Kind, s.ID, s.Title, s.Subtitle, s.CoverURL)
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

// Unsave forgets an artist or an album.
func (d *DB) Unsave(kind, id string) error {
	if _, err := d.Exec(`DELETE FROM saved_items WHERE kind = ? AND id = ?`, kind, id); err != nil {
		return fmt.Errorf("unsave: %w", err)
	}
	return nil
}

// LoadSaved lists what is kept of one kind, newest first.
func (d *DB) LoadSaved(kind string) ([]Saved, error) {
	rows, err := d.Query(`SELECT kind, id, title, subtitle, cover_url FROM saved_items
		WHERE kind = ? ORDER BY added_at DESC, rowid DESC`, kind)
	if err != nil {
		return nil, fmt.Errorf("load saved: %w", err)
	}
	defer rows.Close()
	out := []Saved{}
	for rows.Next() {
		var s Saved
		if err := rows.Scan(&s.Kind, &s.ID, &s.Title, &s.Subtitle, &s.CoverURL); err != nil {
			return nil, fmt.Errorf("load saved: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
