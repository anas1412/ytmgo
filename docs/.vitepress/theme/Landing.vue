<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { withBase } from 'vitepress'
import { cachedVersion, fetchVersion } from './release'

// The page leads with the download that fits the visitor's system.
// Until the browser says what that is (and always in the static page),
// both are offered equally, so nobody is shown the wrong one first.
type Os = 'windows' | 'linux' | 'other'
const os = ref<Os>('other')
const version = ref<string | null>(null)
const copied = ref(false)
const loop = ref<HTMLVideoElement | null>(null)

const RELEASE = 'https://github.com/anas1412/ytmgo/releases/latest/download'
// The site serves the installer itself (docs/public/install.sh): a shorter
// address than the repository's raw file, which the app's updater uses too.
const INSTALL = 'curl -fsSL https://anas1412.github.io/ytmgo/install.sh | bash'

// The command with a break allowed after each slash, so it wraps between
// path parts and never mid-word. It is a constant, so v-html is safe.
const installHtml = INSTALL.replace(/\/(?=[^/])/g, '/<wbr>')

function detect(): Os {
  const ua = navigator.userAgent
  if (/Android|iPhone|iPad|iPod|Mobile|CrOS/i.test(ua)) return 'other'
  if (/Windows/i.test(ua)) return 'windows'
  if (/Linux|X11/i.test(ua)) return 'linux'
  return 'other'
}

async function copy() {
  try {
    await navigator.clipboard.writeText(INSTALL)
    copied.value = true
    setTimeout(() => (copied.value = false), 1800)
  } catch {
    // Clipboard blocked: the command is selectable text, copy it by hand.
  }
}

// The terminal loop is silent and starts by itself; a click pauses it,
// and it stays still for readers who ask for less motion.
function toggleLoop() {
  const v = loop.value
  if (v) v.paused ? v.play() : v.pause()
}

onMounted(() => {
  os.value = detect()
  version.value = cachedVersion()
  fetchVersion().then((v) => v && (version.value = v)).catch(() => {})
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) loop.value?.pause()
})

// Lucide (lucide.dev, ISC), inlined as the rest of the site does: no
// third-party request per visitor.
const features = [
  {
    title: 'Search with no key',
    text: 'Talks to YouTube Music’s own API directly. No account, no API key, no proxy. Exact songs with real metadata and covers.',
    icon: '<path d="m21 21-4.34-4.34" /><circle cx="11" cy="11" r="8" />',
  },
  {
    title: 'Radio that follows you',
    text: 'When the queue runs dry, ytmgo adds what YouTube Music would play next, seeded by what you have actually listened to.',
    icon: '<path d="M4.9 16.1C1 12.2 1 5.8 4.9 1.9" /><path d="M7.8 4.7a6.14 6.14 0 0 0-.8 7.5" /><circle cx="12" cy="9" r="2" /><path d="M16.2 4.8c2 2 2.26 5.11.8 7.47" /><path d="M19.1 1.9a9.96 9.96 0 0 1 0 14.1" /><path d="M9.5 18h5" /><path d="m8 22 4-11 4 11" />',
  },
  {
    title: 'Downloads in one step',
    text: 'Keep any song, or a whole album in its own folder, as M4A or MP3. One at a time, with progress.',
    icon: '<path d="M12 15V3" /><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><path d="m7 10 5 5 5-5" />',
  },
  {
    title: 'Playlists of your own',
    text: 'Make them, reorder them, or paste a YouTube or Spotify playlist link and keep it as it is.',
    icon: '<path d="M21 15V6" /><path d="M18.5 18a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z" /><path d="M12 12H3" /><path d="M16 6H3" /><path d="M12 18H3" />',
  },
  {
    title: 'Favourites, follows, likes',
    text: 'Heart songs, follow artists, like albums. Your history and favourites stay on your computer.',
    icon: '<path d="M2 9.5a5.5 5.5 0 0 1 9.591-3.676.56.56 0 0 0 .818 0A5.49 5.49 0 0 1 22 9.5c0 2.29-1.5 4-3 5.5l-5.492 5.313a2 2 0 0 1-3 .019L5 15c-1.5-1.5-3-3.2-3-5.5" />',
  },
  {
    title: 'Synced lyrics',
    text: 'The current line follows the song. Lyrics are cached locally, so a replay is instant.',
    icon: '<path d="M9 18V5l12-2v13" /><path d="m9 9 12-2" /><circle cx="6" cy="18" r="3" /><circle cx="18" cy="16" r="3" />',
  },
  {
    title: 'Your own music, too',
    text: 'Point ytmgo at your music folders and they show in the Library beside your downloads.',
    icon: '<line x1="22" x2="2" y1="12" y2="12" /><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" /><line x1="6" x2="6.01" y1="16" y2="16" /><line x1="10" x2="10.01" y1="16" y2="16" />',
  },
  {
    title: 'Artist and album pages',
    text: 'Open whoever made a song: their top songs, then the whole discography. One song becomes a catalogue.',
    icon: '<circle cx="12" cy="8" r="5" /><path d="M20 21a8 8 0 0 0-16 0" />',
  },
  {
    title: 'Last.fm and Discord',
    text: 'Scrobble what you hear and show it in your Discord status. Last.fm connects in your browser; no password is typed into the app.',
    icon: '<path d="M4.9 19.1C1 15.2 1 8.8 4.9 4.9" /><path d="M7.8 16.2c-2.3-2.3-2.3-6.1 0-8.5" /><circle cx="12" cy="12" r="2" /><path d="M16.2 7.8c2.3 2.3 2.3 6.1 0 8.5" /><path d="M19.1 4.9C23 8.8 23 15.1 19.1 19" />',
  },
]

// Only what is true of each app today.
const rows: { what: string; win: string; linux: string }[] = [
  { what: 'Runs on', win: 'Windows 10 and 11', linux: 'Linux, x86_64 and arm64' },
  { what: 'You use it with', win: 'Mouse, with keyboard shortcuts', linux: 'Keyboard first, with full mouse support' },
  { what: 'Drag and drop, right-click menu', win: 'Yes', linux: 'No' },
  { what: 'Home with new releases from artists you follow', win: 'Yes', linux: 'No' },
  { what: 'Playlists, favourites, lyrics, downloads', win: 'Yes', linux: 'Yes' },
  { what: 'Last.fm, Discord, your own music folders', win: 'Yes', linux: 'Yes' },
  { what: 'Visualizer and album art in the terminal', win: 'No', linux: 'Yes' },
  { what: 'Updates', win: 'From inside the app, in one click', linux: 'The install script, or your AUR helper' },
  { what: 'Needs installing first', win: 'Nothing, yt-dlp comes with it', linux: 'mpv, yt-dlp and ffmpeg, which the script offers to install' },
]
</script>

<template>
  <div class="lp">
    <!-- ── Hero ─────────────────────────────────────────────────────── -->
    <section class="hero">
      <div class="hero-text">
        <h1>YouTube Music, without the browser.</h1>
        <p class="lede">
          A fast, free YouTube Music client. No account and no API key. A desktop app for Windows,
          a terminal app for Linux. Search, queue, download, read the lyrics, keep your playlists.
        </p>

        <div class="cta">
          <template v-if="os === 'linux'">
            <div class="cmd" role="group" aria-label="Install command">
              <code v-html="installHtml"></code>
              <button type="button" @click="copy">{{ copied ? 'Copied' : 'Copy' }}</button>
            </div>
            <a class="btn ghost" href="#windows">Get the Windows app</a>
          </template>
          <template v-else-if="os === 'windows'">
            <a class="btn primary" :href="`${RELEASE}/ytmgo-gui-setup.exe`">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 15V3" /><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><path d="m7 10 5 5 5-5" /></svg>
              Download for Windows
            </a>
            <a class="btn ghost" href="#linux">Get the Linux app</a>
          </template>
          <template v-else>
            <a class="btn primary" href="#windows">Get it for Windows</a>
            <a class="btn ghost" href="#linux">Get it for Linux</a>
          </template>
        </div>

        <p class="fine">
          <template v-if="os === 'windows'">Windows 10 and 11, 64-bit. </template>
          <template v-else-if="os === 'other'">Runs on Windows and Linux. </template>
          <template v-if="version">Version {{ version.replace(/^v/, '') }}. </template>
          Free and open source, under the MIT licence.
        </p>
      </div>

      <div class="hero-media">
        <img
          class="shot big"
          :src="withBase('/windows-home.jpg')"
          width="1600"
          height="1000"
          alt="ytmgo for Windows: Home with recent albums, recommendations and new releases, the queue beside it and the player below"
        />
        <img
          class="shot small"
          :src="withBase('/tui-app.jpg')"
          width="1155"
          height="692"
          alt="ytmgo in a terminal: an album, the queue, a visualizer and lyrics"
        />
      </div>
    </section>

    <!-- ── The two apps ─────────────────────────────────────────────── -->
    <section class="apps" aria-labelledby="apps-title">
      <h2 id="apps-title">One library, two ways in</h2>
      <p class="section-lede">Pick the one that fits how you work. Both play the same music.</p>

      <div class="app-grid">
        <article id="windows" class="app">
          <div class="app-media">
            <img :src="withBase('/windows-album.jpg')" width="1600" height="1000" loading="lazy" alt="ytmgo for Windows showing an album page with its tracks and the queue" />
          </div>
          <h3>
            <svg viewBox="0 0 24 24" aria-hidden="true"><rect width="20" height="14" x="2" y="3" rx="2" /><path d="M8 21h8" /><path d="M12 17v4" /></svg>
            Desktop app for Windows
          </h3>
          <p>
            A window like the music apps you know: a home page of new releases from artists you follow,
            drag and drop, a right-click menu on every song, and updates in one click.
          </p>
          <div class="actions">
            <a class="btn primary" :href="`${RELEASE}/ytmgo-gui-setup.exe`">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 15V3" /><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><path d="m7 10 5 5 5-5" /></svg>
              Download for Windows
            </a>
          </div>
          <p class="note">
            Windows 10 and 11, 64-bit. It installs for you alone, with no administrator prompt, and brings
            yt-dlp with it. The installer is not signed yet, so SmartScreen warns about an unknown publisher:
            choose <strong>More info</strong>, then <strong>Run anyway</strong>.
            <a :href="withBase('/guide/install#windows')">Windows install notes</a>
          </p>
        </article>

        <article id="linux" class="app">
          <div class="app-media">
            <video
              ref="loop"
              :src="withBase('/hero-loop.mp4')"
              :poster="withBase('/hero-loop-poster.jpg')"
              autoplay
              muted
              loop
              playsinline
              aria-label="ytmgo in a terminal: searching, queueing and playing a song"
              title="Click to pause"
              @click="toggleLoop"
            ></video>
          </div>
          <h3>
            <svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="4 17 10 11 4 5" /><line x1="12" x2="20" y1="19" y2="19" /></svg>
            Terminal app for Linux
          </h3>
          <p>
            Everything from the keyboard, inside your terminal, with mouse support when you want it.
            Album art at full resolution in kitty, Ghostty and WezTerm, and a live spectrum under your results.
          </p>
          <div class="cmd" role="group" aria-label="Install command">
            <code v-html="installHtml"></code>
            <button type="button" @click="copy">{{ copied ? 'Copied' : 'Copy' }}</button>
          </div>
          <p class="note">
            Arch installs from the AUR; everywhere else fetches the release binary and offers to install
            mpv, yt-dlp, ffmpeg and cava.
            <a :href="`${RELEASE}/ytmgo_Linux_x86_64.tar.gz`">x86_64</a>,
            <a :href="`${RELEASE}/ytmgo_Linux_arm64.tar.gz`">arm64</a> and
            <a href="https://aur.archlinux.org/packages/ytmgo-bin">AUR</a> downloads.
            <a :href="withBase('/guide/install')">Install guide</a>
          </p>
        </article>
      </div>
    </section>

    <!-- ── What it does ─────────────────────────────────────────────── -->
    <section class="features" aria-labelledby="features-title">
      <h2 id="features-title">What you get</h2>
      <p class="section-lede">The same things in both apps.</p>
      <ul>
        <li v-for="f in features" :key="f.title">
          <svg viewBox="0 0 24 24" aria-hidden="true" v-html="f.icon"></svg>
          <h3>{{ f.title }}</h3>
          <p>{{ f.text }}</p>
        </li>
      </ul>
    </section>

    <!-- ── Which one ────────────────────────────────────────────────── -->
    <section class="compare" aria-labelledby="compare-title">
      <h2 id="compare-title">Windows app or terminal app</h2>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th scope="col"><span class="sr">What</span></th>
              <th scope="col">Windows app</th>
              <th scope="col">Linux terminal app</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in rows" :key="r.what">
              <th scope="row">{{ r.what }}</th>
              <td :class="{ yes: r.win === 'Yes', no: r.win === 'No' }">{{ r.win }}</td>
              <td :class="{ yes: r.linux === 'Yes', no: r.linux === 'No' }">{{ r.linux }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- ── Close ────────────────────────────────────────────────────── -->
    <section class="close">
      <h2>Free, and yours to read.</h2>
      <p class="section-lede">
        ytmgo is open source under the MIT licence. It keeps your library, history and settings on your
        own computer, and needs no account to use.
      </p>
      <div class="actions">
        <a class="btn primary" :href="`${RELEASE}/ytmgo-gui-setup.exe`">Download for Windows</a>
        <a class="btn ghost" href="#linux">Install on Linux</a>
        <a class="btn ghost" href="https://github.com/anas1412/ytmgo">View the source</a>
      </div>
    </section>
  </div>
</template>

<style>
/* The page's own tokens, over VitePress's, so light and dark both work.
   Violet is the brand; mint marks what is ready to use, as it does in
   the apps themselves. */
.lp {
  --lp-line: var(--vp-c-divider);
  --lp-soft: var(--vp-c-bg-soft);
  --lp-text: var(--vp-c-text-1);
  --lp-mid: var(--vp-c-text-2);
  --lp-violet: var(--vp-c-brand-1);
  --lp-mint: #06d6a0;
  --lp-yes: #0a8f6c;
  --lp-shadow: 0 24px 60px -24px rgb(20 10 50 / 45%);

  max-width: 1152px;
  margin: 0 auto;
  padding: 0 24px 48px;
}
.dark .lp {
  --lp-yes: #06d6a0;
  --lp-shadow: 0 24px 60px -20px rgb(0 0 0 / 75%);
}

.lp h1,
.lp h2,
.lp h3 {
  border: 0;
  margin: 0;
  padding: 0;
  letter-spacing: -0.02em;
  color: var(--lp-text);
}
.lp section {
  padding-top: 112px;
}
.lp h2 {
  font-size: 36px;
  line-height: 1.12;
  font-weight: 700;
}
.lp .section-lede {
  max-width: 56ch;
  margin: 14px 0 0;
  color: var(--lp-mid);
  font-size: 17px;
  line-height: 1.6;
}

/* ── Buttons and the install command ─────────────────────────────── */
.lp .btn {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  height: 46px;
  padding: 0 22px;
  border: 1px solid var(--lp-line);
  border-radius: 999px;
  color: var(--lp-text);
  font-size: 15px;
  font-weight: 600;
  text-decoration: none;
  transition: border-color 0.18s, background 0.18s, transform 0.18s;
}
.lp .btn:hover {
  border-color: var(--lp-violet);
}
.lp .btn svg {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.lp .btn.primary {
  border-color: var(--lp-mint);
  background: var(--lp-mint);
  color: #04140f;
}
.lp .btn.primary:hover {
  background: #2fe3b6;
  border-color: #2fe3b6;
  transform: translateY(-1px);
}
.lp .btn:focus-visible,
.lp .cmd button:focus-visible,
.lp video:focus-visible {
  outline: 2px solid var(--lp-violet);
  outline-offset: 3px;
}

.lp .cmd {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  max-width: 100%;
  padding: 5px 5px 5px 18px;
  border: 1px solid var(--lp-line);
  border-radius: 14px;
  background: var(--lp-soft);
}
.lp .cmd code {
  flex: 1;
  min-width: 0;
  /* Shown whole, wrapping where it must, rather than scrolled. */
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  padding: 8px 0;
  background: none;
  color: var(--lp-text);
  font-size: 13.5px;
  line-height: 1.5;
}
.lp .cmd button {
  flex: none;
  height: 36px;
  padding: 0 16px;
  border-radius: 10px;
  background: var(--lp-violet);
  color: #fff;
  font-size: 13.5px;
  font-weight: 600;
  transition: filter 0.18s;
}
.dark .lp .cmd button {
  color: #0a0a0f;
}
.lp .cmd button:hover {
  filter: brightness(1.1);
}

/* ── Hero ─────────────────────────────────────────────────────────── */
.lp .hero {
  display: grid;
  grid-template-columns: minmax(0, 5fr) minmax(0, 6fr);
  gap: 56px;
  align-items: center;
  padding-top: 72px;
}
.lp .hero h1 {
  font-size: 58px;
  line-height: 1.04;
  font-weight: 750;
  letter-spacing: -0.035em;
}
.lp .lede {
  margin: 22px 0 0;
  color: var(--lp-mid);
  font-size: 18px;
  line-height: 1.6;
}
.lp .cta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-top: 32px;
}
.lp .cta .cmd {
  flex: 1 1 100%;
}
.lp .fine {
  margin: 18px 0 0;
  color: var(--lp-mid);
  font-size: 14px;
  line-height: 1.5;
}

.lp .hero-media {
  position: relative;
  /* Room under the big window for the terminal to overlap it. */
  padding-bottom: 12%;
}
.lp .shot {
  display: block;
  height: auto;
  border: 1px solid var(--lp-line);
  border-radius: 12px;
  box-shadow: var(--lp-shadow);
}
.lp .shot.big {
  width: 100%;
}
.lp .shot.small {
  position: absolute;
  left: -9%;
  bottom: 0;
  width: 54%;
}

/* ── The two apps ─────────────────────────────────────────────────── */
.lp .app-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 56px 40px;
  margin-top: 48px;
}
.lp .app {
  display: flex;
  flex-direction: column;
  scroll-margin-top: 96px;
}
.lp .app-media {
  overflow: hidden;
  aspect-ratio: 16 / 10;
  border: 1px solid var(--lp-line);
  border-radius: 12px;
  background: var(--lp-soft);
  box-shadow: var(--lp-shadow);
}
.lp .app-media img,
.lp .app-media video {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.lp .app-media video {
  cursor: pointer;
}
.lp .app h3 {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 28px;
  font-size: 22px;
  font-weight: 700;
}
.lp .app h3 svg {
  width: 22px;
  height: 22px;
  flex: none;
  fill: none;
  stroke: var(--lp-violet);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.lp .app > p {
  margin: 12px 0 0;
  color: var(--lp-mid);
  line-height: 1.65;
}
.lp .app .actions,
.lp .app .cmd {
  margin-top: 22px;
}
.lp .app .cmd {
  align-self: stretch;
}
.lp .app .note {
  margin-top: 16px;
  font-size: 14px;
  line-height: 1.6;
}
.lp .app .note a {
  color: var(--lp-violet);
  font-weight: 500;
}

/* ── Features: a plain list, divided by hairlines ────────────────── */
.lp .features ul {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 40px 40px;
  margin: 48px 0 0;
  padding: 0;
  list-style: none;
}
.lp .features li {
  margin: 0;
  padding-top: 22px;
  border-top: 1px solid var(--lp-line);
}
.lp .features li svg {
  display: block;
  width: 24px;
  height: 24px;
  margin-bottom: 14px;
  fill: none;
  stroke: var(--lp-violet);
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.lp .features h3 {
  font-size: 17px;
  font-weight: 650;
}
.lp .features p {
  margin: 8px 0 0;
  color: var(--lp-mid);
  font-size: 15px;
  line-height: 1.65;
}

/* ── Comparison ───────────────────────────────────────────────────── */
.lp .table-wrap {
  margin-top: 40px;
  overflow-x: auto;
}
.lp table {
  display: table;
  width: 100%;
  min-width: 640px;
  margin: 0;
  border-collapse: collapse;
}
.lp th,
.lp td {
  padding: 16px 20px 16px 0;
  border: 0;
  border-top: 1px solid var(--lp-line);
  background: none;
  text-align: left;
  vertical-align: top;
  font-size: 15px;
  line-height: 1.5;
}
.lp thead th {
  padding-top: 0;
  border-top: 0;
  color: var(--lp-text);
  font-size: 17px;
  font-weight: 700;
}
.lp tbody th {
  width: 34%;
  color: var(--lp-text);
  font-weight: 500;
}
.lp td {
  width: 33%;
  color: var(--lp-mid);
}
.lp td.yes {
  color: var(--lp-yes);
  font-weight: 600;
}
.lp td.no {
  color: var(--lp-mid);
  opacity: 0.7;
}
.lp .sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}

/* ── Close ────────────────────────────────────────────────────────── */
.lp .close {
  margin-top: 112px;
  padding-top: 56px;
  border-top: 1px solid var(--lp-line);
}
.lp .close .actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 28px;
}

/* ── Narrower screens ─────────────────────────────────────────────── */
@media (max-width: 1100px) {
  .lp .hero h1 {
    font-size: 50px;
  }
}
@media (max-width: 959px) {
  .lp section {
    padding-top: 88px;
  }
  .lp .hero {
    grid-template-columns: minmax(0, 1fr);
    gap: 48px;
    padding-top: 40px;
  }
  .lp .hero h1 {
    font-size: 44px;
  }
  .lp .hero-media {
    padding-bottom: 10%;
  }
  .lp .shot.small {
    left: -3%;
  }
  .lp .app-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .lp .features ul {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 639px) {
  .lp {
    padding: 0 20px 32px;
  }
  .lp h2 {
    font-size: 30px;
  }
  .lp .hero h1 {
    font-size: 36px;
  }
  .lp .lede {
    font-size: 16.5px;
  }
  .lp .shot.small {
    display: none;
  }
  .lp .hero-media {
    padding-bottom: 0;
  }
  .lp .features ul {
    grid-template-columns: minmax(0, 1fr);
    gap: 32px;
  }
  .lp .btn {
    width: 100%;
    justify-content: center;
  }
  .lp .cta .btn,
  .lp .close .btn {
    flex: 1 1 100%;
  }
}
@media (prefers-reduced-motion: reduce) {
  .lp .btn,
  .lp .cmd button {
    transition: none;
  }
}
</style>
