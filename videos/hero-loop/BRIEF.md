---
workflow: general-video
flow: automation
storyboard: no
message: "YouTube Music, played from the keyboard, inside your terminal"
destination: website-hero
aspect: 1920x1080
language: en
length: 12s
---

## Intent

Seamless, silent, autoplaying loop for the ytmgo docs site hero, in place of
the terminal screenshot. Concept: "the terminal comes alive" — a stylised
vector rebuild of the ytmgo TUI: `>_y` mark → window opens → search types →
results cascade → Enter → track plays (progress, visualizer, lyrics) → window
folds back into the mark, which is frame 0 again.

## Notes

- User asked Claude to review the passes itself (storyboard board skipped).
- Palette is the app's own `ytmgo` dark theme (internal/tui/theme.go adaptive()).
- Track names and lyrics are invented — no real songs or artists.
- No audio track. Deliver MP4 (H.264) + WebM + poster.
