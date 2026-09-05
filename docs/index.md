---
layout: home

hero:
  name: ytmgo
  # VitePress renders this with v-html, so the break is explicit rather
  # than left to wherever the column happens to run out.
  text: YouTube Music in your terminal<br>Built with Go.
  tagline: Search, download, queue, and play music, all from the keyboard, inside your terminal. No browser, no bloat, no nonsense.
  image:
    src: https://raw.githubusercontent.com/anas1412/ytmgo/main/ytmgo.png
    alt: ytmgo running in a terminal
  actions:
    - theme: brand
      text: Install
      link: /guide/install
    - theme: alt
      text: Keybindings
      link: /guide/keybindings
    - theme: alt
      text: View on GitHub
      link: https://github.com/anas1412/ytmgo

# Icons are Lucide (lucide.dev, ISC), inlined rather than fetched from a
# CDN: no third-party request per visitor, nothing to break offline or
# behind a firewall, and they inherit currentColor so they follow the
# theme. VitePress renders a string icon as raw HTML, which is why these
# are plain strings and not { svg: ... }, a shape it silently ignores.
features:
  - icon: '<svg class="lucide lucide-search" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" > <path d="m21 21-4.34-4.34" /> <circle cx="11" cy="11" r="8" /> </svg>'
    title: Native YouTube Music search
    details: Talks to YouTube Music's own API directly. No key, no login, no proxy. Exact tracks with real metadata.
  - icon: '<svg class="lucide lucide-radio-tower" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" > <path d="M4.9 16.1C1 12.2 1 5.8 4.9 1.9" /> <path d="M7.8 4.7a6.14 6.14 0 0 0-.8 7.5" /> <circle cx="12" cy="9" r="2" /> <path d="M16.2 4.8c2 2 2.26 5.11.8 7.47" /> <path d="M19.1 1.9a9.96 9.96 0 0 1 0 14.1" /> <path d="M9.5 18h5" /> <path d="m8 22 4-11 4 11" /> </svg>'
    title: Autoplay radio
    details: Queue runs dry? ytmgo queues what YouTube Music itself would play next, seeded by what you have actually listened to.
  - icon: '<svg class="lucide lucide-download" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" > <path d="M12 15V3" /> <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /> <path d="m7 10 5 5 5-5" /> </svg>'
    title: Download in one key
    details: Press x on any track and it downloads, one at a time, with a progress bar, on a page of its own.
  - icon: '<svg class="lucide lucide-heart" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" > <path d="M2 9.5a5.5 5.5 0 0 1 9.591-3.676.56.56 0 0 0 .818 0A5.49 5.49 0 0 1 22 9.5c0 2.29-1.5 4-3 5.5l-5.492 5.313a2 2 0 0 1-3 .019L5 15c-1.5-1.5-3-3.2-3-5.5" /> </svg>'
    title: Favourites, history, library
    details: f to bookmark, a listening-history page, and a filterable library of everything already on disk.
  - icon: '<svg class="lucide lucide-mouse-pointer-click" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" > <path d="M14 4.1 12 6" /> <path d="m5.1 8-2.9-.8" /> <path d="m6 12-1.9 2" /> <path d="M7.2 2.2 8 5.1" /> <path d="M9.037 9.69a.498.498 0 0 1 .653-.653l11 4.5a.5.5 0 0 1-.074.949l-4.349 1.041a1 1 0 0 0-.74.739l-1.04 4.35a.5.5 0 0 1-.95.074z" /> </svg>'
    title: Full mouse support
    details: Click tabs, click panels, click the seek bar to jump. Most terminal apps cannot do this.
  - icon: '<svg class="lucide lucide-play" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" > <path d="M5 5a2 2 0 0 1 3.008-1.728l11.997 6.998a2 2 0 0 1 .003 3.458l-12 7A2 2 0 0 1 5 19z" /> </svg>'
    title: Media keys
    details: Play, pause and skip with your keyboard's media keys, or from your desktop's media widget, over MPRIS.
  - icon: '<svg class="lucide lucide-music-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" > <path d="M9 18V5l12-2v13" /> <path d="m9 9 12-2" /> <circle cx="6" cy="18" r="3" /> <circle cx="18" cy="16" r="3" /> </svg>'
    title: Synced lyrics
    details: Press y for lyrics beside the queue, the current line following the song. Cached locally, so replays are instant.
  - icon: '<svg class="lucide lucide-disc-3" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" > <circle cx="12" cy="12" r="10" /> <path d="M6 12c0-1.7.7-3.2 1.8-4.2" /> <circle cx="12" cy="12" r="2" /> <path d="M18 12c0 1.7-.7 3.2-1.8 4.2" /> </svg>'
    title: Whole albums
    details: Search albums, preview the tracklist, queue it or download the lot into its own numbered folder.
  - icon: '<svg class="lucide lucide-palette" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" > <path d="M12 22a1 1 0 0 1 0-20 10 9 0 0 1 10 9 5 5 0 0 1-5 5h-2.25a1.75 1.75 0 0 0-1.4 2.8l.3.4a1.75 1.75 0 0 1-1.4 2.8z" /> <circle cx="13.5" cy="6.5" r=".5" fill="currentColor" /> <circle cx="17.5" cy="10.5" r=".5" fill="currentColor" /> <circle cx="6.5" cy="12.5" r=".5" fill="currentColor" /> <circle cx="8.5" cy="7.5" r=".5" fill="currentColor" /> </svg>'
    title: Eleven themes
    details: Borrow your terminal's own colours, use ytmgo's, or pick a full scheme such as gruvbox, nord, dracula or catppuccin.
---

## Install in one line

```bash
curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/install.sh | bash
```

On Arch it installs from the AUR; everywhere else it fetches the release
binary, installs a desktop entry, and pulls in `mpv`, `yt-dlp`, `ffmpeg`
and `cava` with your package manager.

See the [install guide](/guide/install) for per-distro detail, building
from source, and how to remove it again.
