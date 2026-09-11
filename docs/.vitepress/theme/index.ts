import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import './custom.css'

// Clicking the terminal capture opens it full size. Written against the
// DOM rather than pulled from a package: it is one overlay and two ways
// to dismiss it, and a dependency for that would be a poor trade.
//
// Guarded on `document` because enhanceApp also runs during the static
// build, where there is no DOM to attach anything to.
function mountLightbox() {
  if (typeof document === 'undefined') return

  const open = (src: string, alt: string) => {
    const box = document.createElement('div')
    box.className = 'ytmgo-lightbox'
    // Announced to assistive tech, and focusable so Escape reaches it
    // even when the click came from a mouse.
    box.setAttribute('role', 'dialog')
    box.setAttribute('aria-modal', 'true')
    box.setAttribute('aria-label', alt || 'Screenshot')
    box.tabIndex = -1

    const img = document.createElement('img')
    img.src = src
    img.alt = alt
    box.appendChild(img)

    const close = () => {
      box.remove()
      document.removeEventListener('keydown', onKey)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close()
    }

    box.addEventListener('click', close)
    document.addEventListener('keydown', onKey)
    document.body.appendChild(box)
    box.focus()
  }

  // One delegated listener, so it keeps working across client-side
  // navigation without re-binding on every route change.
  document.addEventListener('click', (e) => {
    const el = e.target as HTMLElement | null
    if (!el || el.tagName !== 'IMG') return
    if (!el.closest('.VPHero .image')) return
    e.preventDefault()
    open((el as HTMLImageElement).src, (el as HTMLImageElement).alt)
  })
}

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
const REPO = 'anas1412/ytmgo'
const CACHE_KEY = 'ytmgo:release'

function cachedVersion(): string | null {
  try {
    const { version } = JSON.parse(localStorage.getItem(CACHE_KEY) ?? '{}')
    return typeof version === 'string' ? version : null
  } catch {
    return null
  }
}

async function fetchVersion(): Promise<string | null> {
  // No custom headers: an Accept of application/vnd.github+json is not
  // CORS-safelisted, so it turns this into a preflighted request — an
  // extra round trip to be told what the default already returns.
  const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`)
  if (!res.ok) return null
  const { tag_name } = await res.json()
  if (typeof tag_name !== 'string') return null
  try {
    localStorage.setItem(CACHE_KEY, JSON.stringify({ at: Date.now(), version: tag_name }))
  } catch {
    // Private browsing, or storage full. The version still renders.
  }
  return tag_name
}

function showVersion(version: string) {
  // Every copy: the bar has one, the mobile menu another.
  document
    .querySelectorAll<HTMLAnchorElement>(`a[href^="https://github.com/${REPO}/releases"]`)
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
  enhanceApp() {
    mountLightbox()
    mountReleaseVersion()
  },
} satisfies Theme
