The first release of ytmgo for Windows: a desktop app on the same code as
the terminal app.

**Install:** download `ytmgo-gui-setup.exe` below and run it. It installs
for you alone, with no administrator prompt. The installer is not signed
yet, so Windows SmartScreen warns about an unknown publisher: choose
**More info**, then **Run anyway**. `ytmgo-gui-setup.exe.sha256` is its
checksum.

**What it does**

- Search YouTube Music as you type, or paste a YouTube or Spotify
  playlist link. Recommendations from what you've played.
- A queue you reorder by dragging; shuffle, repeat, and autoplay when it
  runs out.
- Album and artist pages, favourites, history.
- Downloads (M4A or MP3, with covers), a library of your own folders,
  and songs that play from disk whenever you have the file.
- Synced lyrics that follow the song; click a line to jump to it.
- Last.fm scrobbling, Discord status, media keys, and the keyboard
  shortcuts of the terminal app (press `?`).

**Good to know**

- yt-dlp comes with the installer. ffmpeg, which downloads need, is
  fetched the first time you download something.
- Your data lives in `%LOCALAPPDATA%\ytmgo`; uninstalling keeps it.
- This is 0.1.0: please report anything that doesn't work at
  https://github.com/anas1412/ytmgo/issues.
