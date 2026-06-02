# Modularization Plan: ytmgo-tui TUI Package

**Goal:** Break up the god files (`update.go` 1208 lines, `view.go` 1234 lines, `model.go` 898 lines) into focused, single-responsibility files — without changing any behavior, signatures, or types.

**Strategy:** Pure file splits. No refactoring, no renames, no restructuring. Each step is just `git mv` a group of functions into a new file. `go build` must pass after every step.

---

## Package Snapshot (before)

| File | Lines | Role |
|------|-------|------|
| `update.go` | 1208 | God file — all message/event handling |
| `view.go` | 1234 | God file — all rendering |
| `model.go` | 898 | Struct + commands + layout + library matching + confirmation |
| `styles.go` | 399 | Style vars + 2 utility renders |
| `keys.go` | 209 | Key binding definitions |
| **Total** | **3948** | |

---

## Splitting Plan (9 steps, safe order)

### Step 1: `commands.go` — from `model.go`
**Move:** All `tea.Cmd` factory functions (~120 lines)

- `searchCmd()`
- `fetchRecommendationsCmd()`
- `scanLibraryCmd()`
- `positionCmd()`
- `endedCmd()`
- `tickCmd()`
- `playerTickCmd()`
- `saveSettingsCmd()`
- `downloadCmd()`
- Constants: `progressTickInterval`, `playerTickInterval`

**Risk:** 🟢 none — only reference external packages, never import tui internals.

---

### Step 2: `keyboard.go` — from `update.go`
**Move:** All `tea.KeyMsg` handling + `handleGlobalKey()` (~485 lines)

- The entire `case tea.KeyMsg:` block (~650 lines)
- `(m *Model) handleGlobalKey()`
- All key-specific inline handlers

**Note:** `playSelectedQueueItem()` stays in `update.go` — also called by mouse and async handlers.

**Risk:** 🟢 none — same package, only calls Model methods.

---

### Step 3: `async.go` — from `update.go`
**Move:** All async message handlers (~180 lines)

- `case SearchResultsMsg:`
- `case RecommendationsMsg:`
- `case LibraryScanMsg:`
- `case SettingsSavedMsg:`
- `case DownloadProgressMsg:`
- `case PositionMsg:`
- `case SongEndedMsg:`

**Risk:** 🟢 none — only reference model fields and external packages.

---

### Step 4: `mouse.go` — from `update.go`
**Move:** Mouse event handling (~110 lines)

- `case tea.MouseMsg:` handlers (wheel up/down, left click)
- `(m Model) handleClick(x, y int) Model`
- Constants: `clickHeaderLines`, `clickPanelStartY`, `clickPlayerHeight`

**Risk:** 🟢 none.

---

### Step 5: `ticks.go` — from `update.go`
**Move:** Periodic tick handlers (~50 lines)

- `case tickMsg:`
- `case playerTickMsg:`

**Risk:** 🟢 none.

**After Step 5:** `update.go` shrinks from 1208 → ~380 lines (just `Init()`, the `Update()` switch dispatch shell, and `playSelectedQueueItem()`).

---

### Step 6: `library.go` — from `model.go`
**Move:** Library matching & conversion functions (~120 lines)

- `searchResultToTrack()`
- `normalizeForMatch()`
- `trackMatchKey()`
- `findLibraryMatch()`
- `(m *Model) resolveTrack()`
- `(m *Model) backfillQueueFromLibrary()`
- `(m Model) filteredLibrary()`

**Risk:** 🟢 none — only depend on `search` and `queue` packages.

---

### Step 7: `confirmation.go` — from `model.go` + `update.go` + `view.go`
**Move:** Confirmation-dialog logic (~130 lines scattered)

From `model.go`:
- `confirmNone`, `confirmClearQueue`, `confirmDeleteTrack` constants
- `isConfirming()`, `startConfirm()`, `clearConfirm()`, `executeConfirmedAction()`

From `update.go`:
- The confirmation routing in the Enter/Esc key handler

From `view.go`:
- `renderConfirmOverlay()`

**Risk:** 🟢 low — groups scattered but self-contained logic.

---

### Step 8: `layout.go` — from `model.go`
**Move:** Layout helpers & filesystem utilities (~180 lines)

- `panelHeight()`, `visibleItems()`, `settingsVisibleItems()`
- `clampSearchOffset()`, `clampLibraryOffset()`, `clampQueueOffset()`, `clampSettingsOffset()`
- `switchPage()`, `startSettingsEdit()`
- `downloadDir()`, `userDataDir()`, `openInOS()`
- `ensureDownloader()`, `ensurePlayer()`

**Risk:** 🟢 none.

---

### Step 9: Split `view.go` into focused rendering files
**Move:** Each render function into its own file by domain (~1200 lines total)

| New file | Contents | Lines |
|----------|----------|-------|
| `header.go` | `renderHeader()` | ~40 |
| `panels.go` | `renderPanels()`, `renderSearchResults()`, `renderLibrary()`, `formatResultRow()`, `renderQueue()`, `formatQueueRow()`, `renderListItemBlock()`, `renderDownloadQueue()` | ~400 |
| `playerbar.go` | `renderPlayerBar()`, `renderControls()` | ~200 |
| `settingspage.go` | `renderSettingsPage()`, `renderSettingsPanels()`, `renderSettingsList()` | ~200 |
| `status.go` | `renderStatus()` | ~20 |
| `help.go` | `renderHelpBar()`, `renderHelpPanel()` | ~60 |
| `overlay.go` | `renderConfirmOverlay()` (already moved in Step 7 if desired) | ~65 |
| `viewutils.go` | `boolStr()`, `padToWidth()`, `truncate()`, `percentage()`, `formatDuration()`, `formatTime()`, `fillHeight()` | ~100 |
| `view.go` | `View()` dispatch shell (shrinks to ~20 lines) | ~20 |

**Risk:** 🟢 none — pure rendering, reads Model fields, writes strings.

---

## Risk Summary

| Concern | Verdict |
|---------|---------|
| Circular imports | 🟢 **Zero risk** — all extracted files stay in `package tui`, only reference external pkgs |
| Broken builds mid-step | 🟢 **None** — each step is a complete, compilable extraction |
| Behavior changes | 🟢 **None** — pure file moves, no signature/type/rename changes |
| Test breakage | 🟢 **No tests exist** in this package (`go test` returns `?`) |

## State (after all 9 steps)

| File | Lines | Role |
|------|-------|------|
| `model.go` | ~410 | Struct fields, `InitialModel()`, `setStatus`/`clearStatus`, `Shutdown`, `startTrackPlayback` |
| `commands.go` | ~120 | All `tea.Cmd` factories |
| `library.go` | ~120 | Library matching |
| `confirmation.go` | ~70 | Confirmation dialog |
| `layout.go` | ~180 | Layout + filesystem helpers |
| `update.go` | ~380 | `Init()`, `Update()` dispatch, `playSelectedQueueItem()` |
| `keyboard.go` | ~485 | All key handlers |
| `async.go` | ~180 | All async message handlers |
| `mouse.go` | ~110 | Mouse handlers |
| `ticks.go` | ~50 | Tick handlers |
| `view.go` | ~20 | `View()` dispatch |
| `header.go` | ~40 | Header render |
| `panels.go` | ~400 | Panel content renders |
| `playerbar.go` | ~200 | Player bar renders |
| `settingspage.go` | ~200 | Settings page renders |
| `status.go` | ~20 | Status bar render |
| `help.go` | ~60 | Help renders |
| `viewutils.go` | ~100 | View utilities |
| `styles.go` | 399 | (unchanged) |
| `keys.go` | 209 | (unchanged) |
| **Total** | **~3948** | (same total lines, now organized) |

**Max file size after split:** `keyboard.go` ~485 lines (down from 1208).
