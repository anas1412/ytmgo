import { h } from 'vue'
import DefaultTheme from 'vitepress/theme'
import { type Theme } from 'vitepress'
import './custom.css'
import Landing from './Landing.vue'
import { REPO, cachedVersion, fetchVersion } from './release'

// The nav's "Releases" entry shows the current version instead of the
// word. Read in the browser rather than baked in at build time, because
// a built-in value goes stale the moment a release is cut: GitHub will
// not start a workflow from an event raised with the default
// GITHUB_TOKEN, so the release workflow publishing a tag cannot trigger
// a docs rebuild. v1.0.0 shipped with the nav still reading v0.14.0.
//
// The last value seen is remembered so a returning reader gets the
// number on the first paint rather than watching "Releases" swap a
// moment later. It is a cache against flicker, not against requests:
// every load still asks, and repaints if the answer differs.
//
// It used to skip the request entirely while the stored value was under
// a day old, which quietly reintroduced the staleness this whole
// mechanism exists to avoid — someone who had read the site that
// morning saw the old version all day after a release.
//
// Any failure leaves the entry reading "Releases", which is what the
// config says and what a reader without JavaScript sees.
function showVersion(version: string) {
  // Every copy: the bar has one, the mobile menu another. Only those:
  // the home page links to release downloads too, and their labels
  // are their own.
  document
    .querySelectorAll<HTMLAnchorElement>(
      `:is(.VPNavBar, .VPNavScreen) a[href^="https://github.com/${REPO}/releases"]`,
    )
    .forEach((a) => {
      const label = a.querySelector('span') ?? a
      if (label.textContent !== version) label.textContent = version
    })
}

function mountReleaseVersion() {
  if (typeof document === 'undefined') return

  let version: string | null = null

  const watch = () => {
    // enhanceApp can run before the body exists, and the mobile menu is
    // built only when the hamburger is first tapped — long after the
    // version lands. So wait for the body, then re-apply whenever nav
    // nodes appear; that covers the menu and client-side navigation.
    if (!document.body) {
      setTimeout(watch, 50)
      return
    }
    let queued = false
    new MutationObserver(() => {
      if (!version || queued) return
      queued = true
      setTimeout(() => {
        queued = false
        if (version) showVersion(version)
      }, 0)
    }).observe(document.body, { childList: true, subtree: true })

    if (version) showVersion(version)
  }

  const got = (v: string | null) => {
    if (!v) return
    version = v
    showVersion(v)
  }

  // The request needs no DOM, so it starts now; only painting waits.
  // The cached value paints first and the fetched one replaces it —
  // both go through got(), and showVersion only touches the label when
  // the text actually changes, so an unchanged version repaints nothing.
  const hit = cachedVersion()
  if (hit) got(hit)
  fetchVersion().then(got).catch(() => {})

  watch()
}

export default {
  extends: DefaultTheme,
  // The home page is one component, in place of the stock hero and
  // feature grid: it shows the download that fits the visitor's system.
  Layout: () => h(DefaultTheme.Layout, null, { 'home-hero-before': () => h(Landing) }),
  enhanceApp() {
    mountReleaseVersion()
  },
} satisfies Theme
