# Keybindings

Everything ytmgo does has a key. The app also shows the ones that matter
where they are used: in panel titles, on the player bar and in the help
bar along the bottom. `z` hides those inline hints once you no
longer need them.

Press `?` at any time to open Settings, which lists the full set.

## Getting around

| Key | Action |
|-----|--------|
| `Tab` | Cycle focus: search → results → queue → search |
| `↑` `↓` / `j` `k` | Move through a list |
| `g` / `G` | Jump to the top / bottom of a list |
| `1` … `6` | Switch page: Stream, Favourites, Library, History, Downloads, Settings |
| `L` | Jump straight to the Library page |
| `?` | Open Settings, which lists every shortcut |
| `esc` | Cancel, or leave an open album |
| `q` / `Ctrl+C` | Quit |

## Playback

| Key | Action |
|-----|--------|
| `Enter` | In results: add to queue. In the queue: play that track |
| `Space` | Play / pause |
| `n` / `→` | Next track |
| `p` / `←` | Previous track, or restart the current one if more than 3s in |
| `l` / `Ctrl+F` | Seek forward 5s |
| `h` / `Ctrl+B` | Seek backward 5s |
| `+` / `=` | Volume up |
| `-` / `_` | Volume down |
| `s` | Toggle shuffle |
| `r` | Cycle repeat: off → one → all |

::: tip Repeat-one and the next key
Repeat-one governs what happens when a track *ends*, not whether you can
move on. `n` always advances, even with repeat-one enabled.
:::

## Queue

| Key | Action |
|-----|--------|
| `d` / `Delete` | Remove the selected track from the queue |
| `D` | Clear the entire queue (press `Enter` to confirm) |
| `Ctrl+↑` / `Ctrl+↓` | Move the selected track up or down |

## Finding music

| Key | Action |
|-----|--------|
| `R` | Refresh recommendations |
| `A` | Switch searching between songs and albums |
| `i` | Open the album the selected track belongs to |
| `a` | Queue every track of the open album |
| `f` | Toggle favourite on the selected track |
| `C` | Clear play history (History page) |

## Downloads

| Key | Action |
|-----|--------|
| `x` | Download the selected track, and jump to the Downloads page |
| `X` | Jump to the Downloads page and back |
| `o` | Open the download directory in your file manager |
| `U` | Check for updates, and confirm the install |

## Panels

| Key | Action |
|-----|--------|
| `v` | Toggle the visualizer beneath the results list |
| `y` | Toggle the lyrics pane beneath the queue. Scroll it with the wheel |
| `z` | Show or hide the inline `[key]` hints |

## Mouse

The mouse works throughout, which is unusual for a terminal app:

- Click a **page tab** in the header to switch pages
- Click a **track** to select it, double-click to play it
- Click the **seek bar** to jump to that point in the song
- Click **prev / play / next**, the shuffle and repeat labels, or anywhere on the **volume bar**
- **Scroll** any list, or the lyrics pane, with the wheel

Your keyboard's media keys and your desktop's media widget also control
playback, over MPRIS.
