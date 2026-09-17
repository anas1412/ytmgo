# Browsing

Search finds a song. From that one song you can reach the record it came
from, everything else its artist has made, and a queue that keeps going
after you stop choosing.

Two keys do all of it: **`a` for the album, `A` for the artist.**
Lowercase is the record, uppercase is the person.

## From a song to its album

Put the cursor on any track — a search result, a queue entry, a
recommendation, a favourite — and press `a`.

```
╭─ ALBUM  [a] queue all songs  [x] download  [esc] releases ──────╮
│ ▇▇▇▇  Random Access Memories                                    │
│ ▇▇▇▇  Daft Punk · 2013                                          │
│ ▇▇▇▇  13 tracks · 1:14:46  ·  [a] queue all songs · [esc] back  │
│                                                                 │
│ 01. Give Life Back to Music          54M plays            4:34  │
│ 02. The Game of Love                 28M plays            5:22  │
```

The album opens with its cover, its year, its running time and every
track in order, each with the number of plays it has. From here:

| Key | Action |
|-----|--------|
| `Enter` | Add one track to the queue |
| `a` | Queue the whole album |
| `x` | Download the whole album into its own numbered folder |
| `esc` | Step back |

Inside an album, `a` has switched from *open* to *queue all* — the one
album thing there is left to do. The panel title says so, so you never
have to remember which meaning applies.

::: tip Local files have no album page
Tracks played from your Library, and rows saved by very old versions,
may not carry the id an album page needs. `a` says so rather than
guessing.
:::

## From a song to its artist

Press `A` on any track and you land on whoever made it.

```
╭─ DAFT PUNK · TOP SONGS  [A] switch  [a] album  [esc] back ──────╮
│ ▇▇▇▇  Daft Punk                                                 │
│ ▇▇▇▇  7.17M subscribers · 20 releases                           │
│ ▇▇▇▇  100 top tracks  ·  [A] releases · [esc] back              │
│                                                                 │
│ 001. Instant Crush (feat. Julian Casablancas)  1.2B plays  5:37 │
│ 002. Get Lucky (Radio Edit)                    1.8B plays  4:09 │
```

An artist page opens on their **hundred most-played tracks**, ordered by
plays, because this is a music player and the first thing wanted is
something to play. Press `A` again to switch to their **releases** —
albums, singles and EPs, newest first.

| Key | On the songs | On the releases |
|-----|--------------|-----------------|
| `A` | Switch to releases | Switch back to the songs |
| `Enter` | Queue that song | Open that release |
| `a` | Open that song's album | — |
| `esc` | Leave the artist | Leave the artist |

The two views share the same header, so the artist's name and photo stay
on screen whichever one you are looking at.

## Everything nests

An album opened from a song is not a dead end. The artist page is
fetched behind it, so `esc` walks back out the way you would expect
rather than dumping you at your search results:

```
search result ──a──▶ album ──esc──▶ releases ──A──▶ top songs ──esc──▶ back
```

This holds no matter how you got in. Press `a` on a track in your queue
and you land in that album *inside* its artist's page, exactly as if you
had walked in through the front door.

::: tip It works on tracks that never stored an artist
Queue and history rows saved before ytmgo recorded artist ids carry only
the album. The album page names its own artist, so `A` finds the way
there anyway.
:::

## Filtering what is on screen

While an artist page is open, typing in the search box **filters the
list in front of you** instead of starting a new search. It filters the
top songs or the releases, whichever you are on, and updates as you
type. `Enter` does nothing special — there is nothing to submit.

`esc` leaves the artist and clears the filter with it, so you never
return to a results list with a query in the box that matches nothing on
it.

## The cursor decides, not the page

`a` and `A` act on **whatever the cursor is on**. With the focus in the
queue — `Tab` moves it there — `A` opens the artist of the highlighted
*queue* track, even with an artist page open beside it. The page on
screen only flips between songs and releases when the cursor is in it.

## Recommendations

Press `R` for a queue of tracks YouTube Music would play next, seeded
from what you have actually listened to. No account, no login — it
reads your local play history.

Autoplay does the same thing automatically: turn it on with `c` (or the
`∞ AUTO` control on the player bar) and ytmgo keeps the queue fed once
you stop adding to it.

Both need something to go on. On a fresh install, play a few things
first.

## Importing a playlist

Paste a playlist link into the search box and press Enter. Its tracks
fill the results list like any search.

```
https://www.youtube.com/playlist?list=...
https://open.spotify.com/playlist/...
```

A YouTube playlist comes across as it is. Spotify has no audio anyone
else can play, so every track is looked up on YouTube Music by name and
matched on length: a couple of seconds apart is the same recording, an
hour apart is somebody's loop of it. Tracks with no match are skipped,
and the status line says how many.

Nothing is queued until you ask. `e` adds every track on screen.

## With the mouse

Everything above works by clicking, too.

- Click a track or a release to select it, double-click to open or play
- Click a page tab in the header
- Scroll any list, or the lyrics pane, with the wheel

## Taking a song elsewhere

| Key | Action |
|-----|--------|
| `u` | Copy the track's link to the clipboard |
| `f` | Favourite it |
| `x` | Download it |

`u` copies a `youtube.com` link, which opens for anyone. If you would
rather hand out `music.youtube.com` links, change **Copy Link As** on the
Settings page — both point at the same recording.
