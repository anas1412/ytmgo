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

export default {
  extends: DefaultTheme,
  enhanceApp() {
    mountLightbox()
  },
} satisfies Theme
