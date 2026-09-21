package library

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ytmgo/internal/queue"
)

// Track is an alias so library tracks are compatible with the queue.
type Track = queue.Track

// CacheEntry is a cached ffprobe result for one file: its length and
// whatever tags it carries. Empty tag fields mean the file has none.
type CacheEntry struct {
	Mtime       int64 // file modification time (unix seconds)
	DurationSec int
	Title       string
	Artist      string
	Album       string
}

// DurationCache maps a file path to its cached probe result. The name
// predates the tags; it is the probe cache.
type DurationCache map[string]CacheEntry

// audioExts is what the scanner picks up. mpv plays all of these.
var audioExts = map[string]bool{
	".mp3": true, ".m4a": true, ".flac": true, ".ogg": true, ".opus": true, ".wav": true,
}

// Scan walks the downloads directory and every extra folder, recursively,
// and returns the audio files it finds as tracks.
//
// Metadata comes from the file's tags when it has any. When it does not,
// what happens depends on whose file it is. ytmgo named its own
// downloads "Artist - Title", so reading that back is not a guess and
// the split stays. A file from anywhere else is somebody's collection,
// and "a - b" could be either way round: the whole filename becomes the
// title and the artist is left blank, which is never wrong.
//
// Durations and tags come from the cache when the file's mtime is
// unchanged; only new or modified files are probed. The second return
// value holds the fresh probe results for the caller to persist.
func Scan(downloads string, extra []string, cache DurationCache) ([]Track, DurationCache, error) {
	var tracks []Track
	updates := DurationCache{}
	seen := map[string]bool{}

	roots := append([]string{downloads}, extra...)
	for i, root := range roots {
		if root == "" {
			continue
		}
		isDownloads := i == 0
		err := filepath.WalkDir(root, func(fpath string, d fs.DirEntry, err error) error {
			if err != nil {
				// An unreadable subfolder should cost its own contents,
				// not the whole scan.
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() || !audioExts[strings.ToLower(filepath.Ext(d.Name()))] {
				return nil
			}
			if seen[fpath] {
				return nil // a folder listed twice, or nested inside another
			}
			seen[fpath] = true

			var mtime int64
			if info, err := d.Info(); err == nil {
				mtime = info.ModTime().Unix()
			}
			ce, ok := cache[fpath]
			if !ok || ce.Mtime != mtime {
				ce = probe(fpath)
				ce.Mtime = mtime
				updates[fpath] = ce
			}

			title, artist, album := ce.Title, ce.Artist, ce.Album
			if title == "" {
				if isDownloads {
					title, artist = parseFilename(d.Name())
				} else {
					title = strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
					artist = ""
				}
			}

			tracks = append(tracks, Track{
				ID:          fpath,
				Title:       title,
				Artist:      artist,
				Album:       album,
				Duration:    formatDuration(ce.DurationSec),
				DurationSec: ce.DurationSec,
				FilePath:    fpath,
				Downloaded:  true,
			})
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("reading library dir %s: %w", root, err)
		}
	}
	if tracks == nil {
		tracks = []Track{}
	}
	return tracks, updates, nil
}

// probe asks ffprobe for the duration and the tags in one call. Tags
// are read from the container and from the streams: MP3, M4A and FLAC
// keep them on the container, but Ogg and Opus keep them on the stream,
// and asking for only the first left every .opus looking untagged.
// Keys are matched case-insensitively — Vorbis comments come back as
// TITLE, not title.
func probe(path string) CacheEntry {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_entries", "format=duration:format_tags=title,artist,album:stream_tags=title,artist,album",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return CacheEntry{}
	}
	var result struct {
		Format struct {
			Duration string            `json:"duration"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			Tags map[string]string `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return CacheEntry{}
	}

	var ce CacheEntry
	if result.Format.Duration != "" {
		var secs float64
		if _, err := fmt.Sscanf(result.Format.Duration, "%f", &secs); err == nil {
			ce.DurationSec = int(secs)
		}
	}
	tagSets := [][]map[string]string{{result.Format.Tags}}
	for _, s := range result.Streams {
		tagSets = append(tagSets, []map[string]string{s.Tags})
	}
	for _, set := range tagSets {
		for _, tags := range set {
			if ce.Title == "" {
				ce.Title = tagValue(tags, "title")
			}
			if ce.Artist == "" {
				ce.Artist = tagValue(tags, "artist")
			}
			if ce.Album == "" {
				ce.Album = tagValue(tags, "album")
			}
		}
	}
	return ce
}

// tagValue finds key in tags regardless of case.
func tagValue(tags map[string]string, key string) string {
	for k, v := range tags {
		if strings.EqualFold(k, key) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// parseFilename tries to extract "Artist - Title" from a filename.
// Falls back to using the whole stem as the title.
func parseFilename(name string) (title, artist string) {
	stem := strings.TrimSuffix(name, filepath.Ext(name))

	// Try "Artist - Title" pattern (most common)
	if idx := strings.Index(stem, " - "); idx > 0 {
		artist = strings.TrimSpace(stem[:idx])
		title = strings.TrimSpace(stem[idx+3:])
		// Clean up common suffixes like (Official Video), (Lyrics), etc.
		title = cleanTitle(title)
		return title, artist
	}

	// Fallback: whole name is the title
	title = cleanTitle(stem)
	return title, ""
}

// cleanTitle removes common suffixes from video titles.
func cleanTitle(t string) string {
	suffixes := []string{
		"(Official Music Video)",
		"(Official Video)",
		"(Official Lyric Video)",
		"(Lyric Video)",
		"(Lyrics)",
		"(Audio)",
		"(Official Audio)",
		"[Official Music Video]",
		"[Official Video]",
		"[Lyrics]",
		"|",
	}
	for _, s := range suffixes {
		if idx := strings.Index(t, s); idx >= 0 {
			t = strings.TrimSpace(t[:idx])
		}
	}
	return t
}

func formatDuration(secs int) string {
	if secs <= 0 {
		return "0:00"
	}
	m := secs / 60
	s := secs % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
