# Command line

Run `ytmgo` with no arguments and you get the player. Give it a
subcommand and it does one thing and exits. Useful for a quick lookup,
a shell alias, or a script.

```bash
ytmgo                    # open the interactive player
ytmgo search homage      # print matching tracks
ytmgo play homage        # play the first match, ctrl+c to stop
ytmgo download homage    # download the first match
```

Quotes are optional. `ytmgo play mild high club homage` works as typed.

## search

Prints what YouTube Music returns, numbered, with artist and duration.
Nothing is queued or downloaded.

```bash
$ ytmgo search bebalee
1. Bebalee  ·  Fairuz  (4:11)
2. Bebalee (Live)  ·  Fairuz  (5:02)
...
```

## play

Plays the first match straight away, without opening the UI. It shows on
Discord the same way the player does. `Ctrl+C` stops it.

```bash
ytmgo play homage
```

## download

Downloads the first match to the folder set on the Settings page, in the
format set there. Both are overridable per run.

```bash
ytmgo download homage                  # settings' folder and format
ytmgo download -f mp3 homage           # force MP3
ytmgo download -l . homage             # into the current directory
ytmgo download -f mp3 -l ~/Music homage
```

Album art is embedded in the file, so downloaded tracks carry their
cover into any other player.

### Whole albums

`-a` switches every subcommand from tracks to albums. Downloading one
creates a folder named after the album, with the tracks numbered so they
sort in order.

```bash
ytmgo search -a currents            # list matching albums
ytmgo download -a currents          # download the album into its own folder
ytmgo download -a -f mp3 -l . currents
```

## Flags

| Flag | Meaning |
|------|---------|
| `-a`, `--album` | Work on albums instead of single tracks |
| `-f`, `--format` | `m4a` (default) or `mp3` |
| `-l`, `--location` | Destination directory. `.` for the current one, created if missing |

::: tip Flags can go anywhere
`ytmgo download bebalee -f mp3` and `ytmgo download -f mp3 bebalee` do
the same thing. Flags after the query used to be swallowed into the
search text; they are lifted out before parsing now.
:::

## Environment

| Variable | Effect |
|----------|--------|
| `YTMGO_INSTALL_DIR` | Where the installer puts the binary |
| `YTMGO_VERSION` | Install a specific release instead of the latest |
| `YTMGO_FORCE` | Reinstall even when the current version is already present |
