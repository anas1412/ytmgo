# Windows migration plan

Goal: **ytmgo on Windows as a GUI only.** A Wails app built from this repo
(no fork), shipped as one installer: `ytmgo-gui-setup.exe`.

The TUI stays what it is: Linux only.
There is no Windows TUI build, zip, or support.

This is the third draft. Earlier drafts assumed things that did not hold up;
they are listed first, and the plan is built around what replaced them.

---

## What earlier drafts got wrong

| Assumed | What the code shows | Consequence |
| --- | --- | --- |
| Move behaviour out of `internal/tui` into a shared `internal/app` first | `internal/tui` has ~135 call sites into `player`, `queue`, `db`, `lastfm`, `downloader`, and its handlers change domain state and UI state together (`m.results`, `m.searchCursor`, `m.setStatus` in one function, as in `async.go`). It is Bubble Tea shaped: `tea.Cmd` and `tea.Msg` throughout. | Extracting it is a rewrite of the TUI's core. Not done. |
| The leaf packages are tangled with the TUI | They are not. `ytmusic`, `search`, `player`, `queue`, `db`, `lastfm`, `downloader`, `lyrics`, `library`, `ytresolve` are separate, tested packages. | The GUI sits directly on them and leaves the TUI untouched. |
| mpv is the player | mpv was chosen because the code already uses it, not because it was compared on Windows. | The GUI's audio engine is decided by a spike (S2). |
| Bundle mpv, yt-dlp, and ffmpeg | Only yt-dlp is unavoidable. ffmpeg is needed only for downloads (`-x`, thumbnail embed) and for reading local tags via `ffprobe`. mpv may not be needed at all. | Install only what is required; fetch the rest on first use. |
| Run the GUI as a local web server, in a browser or an Edge `--app` window | Both feel like a browser: Edge's taskbar icon, F5 reloads, the user's extensions run inside, a local port to secure, closing the window does not stop the app. | Wails: its own exe and window, no port, Go called directly through bindings. |
| Ship a Windows TUI first | Sharp album art would fall back to half blocks (Windows Terminal does not identify as kitty, Ghostty, or WezTerm). The layout measures glyphs like `▶ ♥ ⏮ ⏭ ⇄ ↻ 🔍` with `lipgloss.Width`, and Windows fonts may draw them at another width, misaligning borders. If S2 passes, mpv on Windows would exist only for the TUI. Every TUI change would need Windows testing. | GUI only. The shared groundwork it would have proved is still done, in Phase 1, for the GUI. |

---

## Decisions made

- **Windows target: the GUI only.**
- **Toolkit: Wails v2**, the stable line. Check v3's status when Phase 2
  starts; it adds a tray icon and was still alpha when this was written. Do
  not build on an alpha.
- **Deliverable: the installer only.** One way to install means one
  install layout (tools always beside the app), one update path, and one
  thing to test and support. No portable exe.
- **Order:** the core groundwork first (Phase 1), because the GUI needs it
  whatever the spikes decide.

---

## Architecture

**A Wails app in the same Go module, built on the existing leaf packages.**

```
cmd/ytmgo-gui/         main.go: Wails options, window, bindings
  frontend/            HTML/CSS/JS, plain (no framework, no npm build step)
internal/gui/          the GUI's Go side: the bound methods and the events
internal/proc/         runs external tools: kill-tree on cancel, no console window on Windows
internal/deps/         finds and fetches yt-dlp, ffmpeg (and mpv if S2 needs it)
internal/...           existing packages, shared with the TUI
```

- **No server and no port.** The frontend calls Go methods directly through
  Wails bindings (`window.go.gui.App.Play(id)`), and Go pushes state to the
  frontend with Wails events (`runtime.EventsEmit`). Nothing on the network to
  secure.
- **The GUI builds on the leaf packages and leaves the TUI alone.** The
  orchestration the TUI keeps in `internal/tui` (play flow, queue advance,
  autoplay radio, scrobble timing, history writes) is written once more in
  `internal/gui`, small and in plain Go.
- **Drift is controlled by extraction on duplication.** When the GUI needs a
  rule the TUI already has (the scrobble threshold, which tracks seed the
  radio), move that one pure function into its leaf package, point the TUI at
  it, and commit it separately. Only rules both need move, and only when the
  second one needs them.
- **Playback state lives in Go.** The frontend asks to play, pause, seek, and
  skip. Go owns the queue and what is playing, and records history and
  scrobbles. Which engine makes the sound (S2) sits behind that, so the
  frontend does not care.
- **Every external tool runs through `internal/proc`.** A GUI app on Windows
  flashes a console window for every child process unless it is started with
  `CREATE_NO_WINDOW`, and cancelling must kill the whole tree (yt-dlp spawns
  ffmpeg; the yt-dlp `.exe` is itself a launcher plus a child). One helper
  does both, so no call site can forget.
- **Only `cmd/ytmgo-gui` and `internal/gui` import Wails,** so the TUI binary
  does not grow. Wails enters the shared `go.mod`; that is the price of
  sharing `internal/`.
- **Native pieces Wails provides:** the folder picker (library folders),
  dialogs, opening links, the clipboard, an application menu, single-instance
  locking, and an optional frameless window with a custom title bar in the
  ytmgo palette.

Why not the alternatives: a browser or an Edge `--app` window behaves like a
browser (see above). Fyne and Gio draw their own widgets, which suits a media
UI (art, long lists, drag to reorder) less well than HTML and CSS. A native
WinUI app means rewriting the core in C# or running Go as a sidecar. Electron
ships a whole browser per app; Wails uses the WebView2 already on Windows.

---

## Dependencies on Windows

Nothing can be assumed: no yt-dlp, no ffmpeg, no mpv, no package manager.

| Tool | Used for | Policy |
| --- | --- | --- |
| `yt-dlp` | resolving and downloading every track | **required.** Shipped by the installer, beside the app. Kept fresh. |
| `ffmpeg` / `ffprobe` | download conversion and cover embedding; reading tags from local files | **on demand**, fetched the first time Downloads or the local Library needs it |
| `mpv` | playback, **only if S2 fails** | not shipped unless S2 says so |
| `cava` | the TUI's visualizer | not used. The GUI draws its own spectrum if the audio route allows. |

- **One locator for all of them.** `internal/ytdlp` already prefers a copy
  beside the binary over `PATH`, so a stale system copy cannot win. Generalise
  it into `internal/deps` with the order: beside the exe, then
  `%LOCALAPPDATA%\ytmgo\bin`, then `PATH`. Every caller asks it for a path;
  nothing runs a tool by bare name. The TUI's behaviour on Linux does not
  change.
- **On-demand download.** `deps` fetches a missing tool into
  `%LOCALAPPDATA%\ytmgo\bin` with progress, a pinned version, and a SHA-256
  check; a mismatch aborts and leaves nothing half-installed.
- **A missing tool is a screen, not an error.** The GUI says what is missing
  and offers to fetch it. It never shows `executable file not found`.
- **yt-dlp freshness.** A stale yt-dlp cannot open a single track. Update at
  most once a day, and offer an update when resolving fails.
- **Reading tags without ffprobe.** `internal/library/library.go:128` shells
  out to `ffprobe` for duration and tags. A pure-Go tag reader would make the
  local Library work with no ffmpeg (S5).
- **Licences.** mpv and ffmpeg are GPL/LGPL depending on the build. Bundling
  means shipping licence texts and a source offer; downloading from the
  upstream project's own builds avoids redistributing them. Check yt-dlp's
  terms for the exact binary you ship.

---

## Phase 0: spikes (go/no-go)

Each spike is a small throwaway that answers one question; write the result
into this file. They need a Windows machine. Phase 1 does not wait for them:
its work is needed whatever they decide.

| Spike | Question | Pass | If it fails |
| --- | --- | --- | --- |
| S1. yt-dlp on Windows | Does `yt-dlp.exe` resolve and play YouTube Music tracks as the Linux build does? Does it need a JavaScript runtime or anything else installed? | resolves with nothing extra | add the missing piece to the dependency list |
| S2. WebView2 audio | In a Wails window, does `<audio>` play a stream URL from yt-dlp: starts, seeks, long tracks, URL expiry, no 403s? | plays reliably for 20 varied tracks | serve the audio through a Wails asset-server handler in Go (can set headers and refresh the URL, still no port); if still bad, drive mpv through `internal/player`, which then needs the named-pipe port (below) |
| S3. Media keys | Does the Media Session API in WebView2 show the title and art in the Windows overlay and take media keys? | works | a small SMTC binding under `//go:build windows` |
| S4. Wails builds | Does `wails build -platform windows/amd64` produce a working exe from Linux with `CGO_ENABLED=0`? Does the NSIS installer build in CI and install WebView2 when it is missing? | all three behave | build on a `windows-latest` runner instead |
| S5. Tag reading | Does a pure-Go reader cover MP3, M4A, FLAC, Ogg, and Opus? | yes | keep ffprobe, on demand |

S1 gates the GUI. S2 decides whether mpv exists on Windows at all.

**S2, on Linux: passed.** `<audio>` plays the stream yt-dlp gives
(`yt-dlp -f bestaudio --get-url`: Opus in WebM, itag 251, valid about six
hours, byte ranges served) in WebKitGTK, played in the real window. Two
findings: resolving takes about 4.5s, so the next track should be
resolved ahead of time; and Google's image servers reset the connection
when the referer is the app's origin, so the page sends none (the audio
servers did not mind). Still to confirm in WebView2 on Windows, the
engine that ships.

**S4, first half: passed.** `wails build -platform windows/amd64` builds a
Windows GUI exe (PE32+, 11.8 MB) on Linux with no cgo. Still open: the NSIS
installer (needs `makensis`), and the exe on a machine without WebView2.

**If S2 falls back to mpv:** `internal/player/player.go:85` builds a Unix
socket path and `:313` dials `"unix"`. mpv on Windows takes a named pipe
(`\\.\pipe\ytmgo-mpv-<pid>`) for `--input-ipc-server`. Split both into
build-tagged files and dial with `gopkg.in/natefinch/npipe.v2`, already in
`go.sum` through rich-go. mpv joins `deps`.

---

## Phase 1: core groundwork for Windows

Only what the GUI needs from the shared packages. SQLite is
`modernc.org/sqlite` (pure Go, no CGO).

1. **`internal/proc`.** `internal/ytresolve/resolver.go:51` and
   `internal/downloader/downloader.go:102` use `Setpgid` and `syscall.Kill`,
   which do not exist on Windows; this is the only thing stopping
   `GOOS=windows CGO_ENABLED=0 go build ./...` today. Move them into one
   helper with Unix and Windows files. On Unix it keeps today's process-group
   kill. On Windows it starts children with `CREATE_NO_WINDOW` and puts each
   tool in its own Job Object set to kill on close. (The first version used
   `taskkill /T`, which imitated Unix process groups. It missed a child
   whose parent had already exited, and left the tools running if ytmgo
   crashed. The Job Object covers cancel, leftover children, and crashes,
   and the tests fail without it.)
   → verify: the Windows cross-build succeeds; a Unix test shows cancelling
   kills a grandchild; Windows tests show that cancel kills a child and that
   `Wait` kills an orphan whose parent already exited (under Wine; both fail
   with the job disabled); existing tests pass. **Done.**
2. **Route the remaining tool calls through `internal/proc`:** `ffprobe` in
   `internal/library`, and `mpv` only if S2 falls back to it. Otherwise each
   one flashes a console window in the GUI. **Done for ffprobe.**
3. **Paths.** `internal/settings` and `internal/db` resolved to
   `~\.local\share` on Windows. Now `%LOCALAPPDATA%` (the database, the log,
   and default downloads), checked by a Windows test under Wine. **Done.**
4. **`internal/deps`:** the locator first, with `internal/ytdlp` folded into
   it and `downloader`, `library`, and `ytresolve` switched over; then the
   on-demand download with pinned versions and checksums.
   → verify: with a tool both beside the exe and on `PATH`, the one beside the
   exe wins; a checksum mismatch aborts cleanly; the TUI on Linux behaves as
   before. **Done**, tested on Linux and under Wine:
   - `internal/deps` replaces `internal/ytdlp`. It looks beside the exe,
     then `BinDir()` (`%LOCALAPPDATA%\ytmgo\bin`), then `PATH`, and adds
     `.exe` on Windows. yt-dlp, ffmpeg, ffprobe, and mpv all resolve through it.
   - yt-dlp finds ffmpeg through `PATH` by itself, which ytmgo's own copy
     is not on in Windows; downloads now pass `--ffmpeg-location` when the
     copy is ytmgo's.
   - `deps.Fetch` downloads a pinned build (`pins_windows.go`: yt-dlp
     2026.08.19, ffmpeg 9.0.2 essentials from gyan.dev), checks its
     SHA-256 before installing, extracts ffmpeg and ffprobe from one zip,
     and leaves nothing behind on any failure.
   - Not yet run: a real download end to end (the tests use a local
     server), and whether gyan's essentials build encodes MP3 (it needs
     libmp3lame). Both belong to the first Windows run.
   - The missing-tool screen that calls `Fetch` is GUI work (Phase 2).
5. **CI.** Add `windows/amd64` to the cross-build loop
   (`.github/workflows/ci.yml:52`), and a `windows-latest` job running
   `go test` on the packages the GUI uses. Tests that shell out to Unix
   tools get build tags or a skip.
   Locally, Windows test binaries run under Wine
   (`GOOS=windows go test -c`, then `wine pkg.test.exe`). **Done:** CI
   cross-builds `windows/amd64` and runs a `windows-latest` job over
   `internal/...` minus `tui` (no `-race`, which needs cgo on Windows).
   Under Wine every package in it passes. What the Windows runs found:
   - `ytdlp` looked for `yt-dlp`, not `yt-dlp.exe`: fixed in step 4.
   - `db` tests isolated themselves with `HOME` and `XDG_DATA_HOME`, which
     Windows ignores, so they wrote to the real
     `%LOCALAPPDATA%\ytmgo\ytmgo.db`. They now fake `LOCALAPPDATA` and
     `USERPROFILE` too.
   - `settings` unescaped `\ ` in library folders, which on Windows
     renamed `D:\Music\ Live` to `D:\Music Live`. The unescape is
     Unix-only now, with a Windows test; the Unix drag-and-drop tests skip
     on Windows.
   - `ytresolve` fakes yt-dlp with a `#!/bin/sh` script, so that test
     skips on Windows; running the command is covered by `proc`'s test.

Exit: every shared package the GUI imports builds and passes its tests on
Windows, and can find or fetch the tools it needs.

---

## Phase 2: Wails skeleton and a minimal UI

Develop on Linux with `wails dev`, which hot-reloads the frontend and can
also open it in a normal browser for inspection. Wails on Linux needs
WebKitGTK, which is a development-machine dependency only.

1. `wails init` into `cmd/ytmgo-gui`, then strip it to vanilla HTML/JS. Set
   the app name, icon (`ytmgo-icon.png`), and window size. **Done**, by hand
   rather than `wails init`, which would have made a separate module:
   - `cmd/ytmgo-gui` (Wails entry, `wails.json`, `frontend/index.html`) and
     `internal/gui` (the bound `App`; it does not import Wails, so it builds
     and tests anywhere).
   - The GUI builds on Windows, and on Linux only with `-tags webkit2_41`
     (WebKitGTK 4.1). The default Linux build, vet and tests never see it,
     so CI needs neither cgo nor WebKit headers.
   - Wails v2.17.0, the latest release (v3 is not out). It raised the
     module to **Go 1.25**; CI now reads the version from `go.mod`, and the
     README, install guide, and AUR `makedepends` say 1.25.
   - The TUI binary grew 19.7 KB (0.09%), from the newer `x/sys`, `x/text`,
     and `termenv` that Wails needs, with no Wails code in it.
   - Verified: `wails dev -tags webkit2_41` runs and the page gets `"dev"`
     back from Go through the binding. Wails' own `ipc.js` logs a
     `reading 'nodes'` error in the browser view; check whether it also
     shows in the real window.
   - Develop with `wails dev -tags webkit2_41` from `cmd/ytmgo-gui`.
2. `internal/gui`: an `App` struct holding the leaf packages, with the
   playback methods (play, pause, seek, volume, next, previous, queue) bound
   to the frontend, and state pushed as events. Back it with the engine S2
   chose: WebView2 `<audio>` reporting `timeupdate`, `ended`, and `error` to
   Go, or mpv through `internal/player`.
3. The missing-tool screen, driven by `internal/deps`.
4. A minimal UI: search, results, play, a player bar. **Search and results
   done; play waits for S2.**
   - `gui.App.Search` wraps `search.Search` with the user's search limit;
     tested without network (trimming, a blank query asks nobody, an unset
     limit falls back to the default). `cmd/ytmgo-gui` opens and closes the
     database, so no lifecycle method is exposed to the page.
   - Design, as asked: modern and Spotify-like, in ytmgo's palette (violet
     for focus and the cursor, mint for active and playing, pink for
     errors), with Windows' own Segoe UI Variable. Sidebar, a main view,
     and a now-playing bar along the bottom. Library, Favourites, History,
     and Downloads are shown but inert until Phase 3 builds them.
   - Feedback: search as you type after a pause (Enter searches at once),
     placeholder rows on the first search, a working line in the field when
     refining, one entrance for each new set of results, cover art fading
     in, and failures that say what happened with **Try again**. All of it
     honours reduced motion.
   - The signature: one violet cursor bar that glides between rows, as the
     TUI's highlight moves. `/`, arrows, `j`/`k`, Enter, and Escape work as
     they do in the terminal app.
   - Checked in `wails dev` in a browser: states, layout, keys, and the
     toast. Not checked: how the motion feels in the real window. The
     preview pane only advances animations when it takes a screenshot.
   - Building the Windows exe from Linux needs `-tags webkit2_41` too:
     Wails generates the JS bindings by building for the host first.
     `wails build -platform windows/amd64 -tags webkit2_41` from
     `cmd/ytmgo-gui`. On a Windows runner no tag is needed.
   → verify: Go tests for `internal/gui` (plain Go, testable without a
   window); search and play work in `wails dev` on Linux and in a built exe on
   Windows; the TUI binary is the same size as before, give or take noise.

---

## Phase 3: GUI feature parity

**The target is the current TUI** (v1.4.0 Stream page), mapped onto a
Spotify-like layout: sidebar, main view, a right panel with Queue and
Lyrics tabs, and the player bar.

| TUI | GUI |
| --- | --- |
| Recommendations from history as the home list (50) | main view when the search is empty |
| Search results; album view (art, year, track count, total length, numbered tracks); artist page (top songs, discography) | main view |
| Queue: count and total time (`30 · 1:21:05`), ▶ on the playing track, ✓ on downloaded ones, ♥ on favourites | right panel, Queue tab |
| Lyrics, synced, current line followed | right panel, Lyrics tab |
| Visualizer | in the player bar or right panel, if the audio route allows (S2) |
| Player: art, title and artist, album (`○ Skiptracing`), next track (`⏭ Tesselation`), Prev / Pause / Next, seek bar with times, `∞ AUTO`, `⇄ SHFL`, `↻ OFF`, volume slider with −/+ | player bar |
| Track keys: `f` favourite, `x` download, `e` queue, `a` album, `A` artist, `u` copy link | the same keys, plus a right-click menu |
| Tabs: Stream, Favs, Library, History, Downloads, Settings | sidebar |
| Quote line; version and update status; key hints, `z` hints, `?` help | status area and a `?` overlay |

In this order, each shippable alone (playback and the full player bar
come first, in Phase 2):

1. Queue: reorder by drag, remove, clear. Autoplay radio when it runs dry.
2. Library (with the native folder dialog), favourites, history.
3. Downloads, with progress. This is where ffmpeg is first fetched on demand.
   Downloads default to `%LOCALAPPDATA%\ytmgo\downloads`, out of sight in
   Explorer, so the page has an **Open downloads folder** button
   (opens it in Explorer with `explorer.exe <path>`) and Settings lets the user
   pick another folder with the native dialog.
4. Album and artist pages; paste a YouTube or Spotify playlist link.
5. Lyrics, synced to playback.
6. Settings: Last.fm connect, Discord, library folders.
7. Spectrum, only if the audio route allows it (a Web Audio analyser works
   with `<audio>`; with mpv it needs a different source).

→ verify each: Go tests in `internal/gui` for the logic, and a manual pass in
the built exe on Windows.

---

## Phase 4: Windows integration

1. Media keys and the Windows overlay, per S3: the Media Session API in the
   frontend, or a small SMTC binding.
2. Single instance (Wails `SingleInstanceLock`): a second launch focuses the
   open window.
3. Optional: a tray icon, if Wails v3 is stable by then.
   → verify on Windows: media keys and the volume flyout drive playback, and
   title and art show in the overlay.

**Status:** 1 and 2 built. Media keys go through the page's Media Session,
with WebView2 started with `--enable-features=HardwareMediaKeyHandling`
(through `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS`), which other WebView2 apps
report the overlay needs. Not yet verified on Windows. No tray icon.

---

## Phase 5: packaging and release

One Windows artefact per release, with a `.sha256` like the existing assets:

| Artefact | What it is | Dependencies |
| --- | --- | --- |
| `ytmgo-gui-setup.exe` | NSIS installer (`wails build -nsis`), per-user, no admin prompt, Start menu entry, uninstaller | installs `yt-dlp.exe` beside the app (and mpv only if S2 needs it); bootstraps WebView2 if missing; ffmpeg on demand |

1. **Pin versions in one file in the repo** (yt-dlp, ffmpeg, and mpv if
   used; builds and checksums), so a release is reproducible and an upstream
   change cannot break it silently.
2. **WebView2.** The installer installs the runtime when it is missing
   (Wails' `-webview2` build flag, set to `download`). Confirm the exact
   behaviour in S4.
3. **Updates.** Check the GitHub release API, as the docs site already does.
   The app downloads and runs the new installer, which replaces it in place.
4. **Signing.** Unsigned exes and installers trigger SmartScreen. Budget for a
   certificate, or accept the warning for early releases. Sign the installer
   and the app exe inside it.
5. **Release workflow.** A Windows job in `release.yml` and `nightly.yml`
   that builds the installer (cross-compiled from Linux if S4 passes,
   otherwise on `windows-latest`).
6. **Optional.** winget and Scoop manifests, once a release is stable.

**Status (0.1.0):** the installer is `cmd/ytmgo-gui/build/windows/installer/
project.nsi`, Wails' template in its per-user mode, plus `yt-dlp.exe`
(`cmd/fetchdep`, from the pins in `internal/deps`). Uninstalling removes
`%LOCALAPPDATA%\ytmgo\bin` too. `windows-installer.yml` builds it on
`windows-latest`; `release.yml` calls it for every `v*` tag and publishes
the installer in the same release as the Linux binaries, one version for
both apps, so the "Latest" release always carries both. Run by hand it
only builds an artifact. (0.1.0 went out alone as `gui-v0.1.0`, a
pre-release, before this.)
Not done: updates (3), signing (4), winget and Scoop (6).

→ verify, on a clean Windows VM:
- the installer gives a working app, a Start menu entry, and a clean uninstall
  that removes the app and `%LOCALAPPDATA%\ytmgo\bin` and leaves the user's
  library and settings alone;
- downloading a track fetches ffmpeg on demand;
- no console window ever flashes;
- an update from N-1 to N works.

---

## Phase 6: docs and site

1. A Windows section in `docs/guide/install.md`: the installer and
   uninstalling. Say plainly that the terminal app is Linux only.
2. The README install section.
3. Optionally, a GUI screenshot or loop on the site.

---

## Backlog: GUI features requested after 1.5.0

- ~~**Home:** a Home button and a home screen, with recommendations on it.~~ Done:
  Home is first in the sidebar (`1`), YouTube Music's signed-out home
  (`ytmusic.HomeFeed`, its moods and shelves), quick tiles and your
  recommendations; Search shows moods to browse when empty. Playlists of
  your own have a placeholder in the sidebar.
- ~~**Clickable links everywhere:**~~ Done in the GUI: a song's title opens
  its album, its artist the artist's page, its album name the album, in
  every list, the queue, the player bar and song cards. Songs saved
  without page ids (history, older favourites, library files) are found
  on YouTube Music first (`Locate`, once per song).
- **Follow and like:** follow artists, and like albums.
- **Favourites tabs:** Songs, Artists and Albums.
- **Playlists:** the logic and the UI. A pasted YouTube or Spotify playlist link, once its songs
  show in Search, gets a **Save as playlist** button that saves them under
  the playlist's own name.
- **Right panel:** song and artist details after the Lyrics tab. Opened by clicking the cover in the player bar.
- ~~**Queue order, in both apps:**~~ Done, Spotify's way, in both: the
  track playing first, then what comes next in play order (shuffle
  included), then what has played. Adding a song puts it next, after
  songs added before it (`queue.AddNext`, `queue.Order`).

## Open decisions

- **Audio engine** (S2): WebView2 `<audio>`, a Go asset handler in front of
  it, or mpv. Decided by the spike, not by preference.
- **Wails v2 or v3?** v2 unless v3 is stable when Phase 2 starts.
- **Fetch pinned tool builds from upstream, or mirror them as ytmgo release
  assets?** Mirroring removes a dependency on someone else's servers and adds
  storage and a licence-notice duty.
- **windows/arm64 now or later?**
- **A code-signing certificate before 1.0?**

## Out of scope

- A Windows TUI.
- A Linux or macOS GUI. Wails builds for both, so it is possible later, but it
  is not a goal and is not tested.
- Refactoring the TUI onto `internal/gui`.
