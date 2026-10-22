// The latest release's tag, read in the browser: the nav entry and the
// home page both show it. See index.ts for why it is not baked in.
export const REPO = 'anas1412/ytmgo'
const CACHE_KEY = 'ytmgo:release'

export function cachedVersion(): string | null {
  try {
    const { version } = JSON.parse(localStorage.getItem(CACHE_KEY) ?? '{}')
    return typeof version === 'string' ? version : null
  } catch {
    return null
  }
}

export async function fetchVersion(): Promise<string | null> {
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

