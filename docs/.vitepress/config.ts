import { defineConfig } from 'vitepress'

const REPO = 'anas1412/ytmgo'

// The latest release tag and the star count, read once at build time and
// baked into the nav. Not fetched in the browser: that would cost every
// visitor a GitHub API call against a 60-per-hour-per-IP limit, to show
// two numbers that barely move.
//
// Freshness comes from the workflow instead — docs.yml rebuilds on a
// published release and on a weekly schedule. A failed fetch (offline,
// rate-limited, API down) must never fail the build, so each falls back
// to an empty string and the nav simply renders without it.
async function githubFacts() {
  const get = async (path: string) => {
    try {
      const res = await fetch(`https://api.github.com/repos/${REPO}${path}`, {
        headers: { accept: 'application/vnd.github+json' },
      })
      return res.ok ? await res.json() : null
    } catch {
      return null
    }
  }

  const [release, repo] = await Promise.all([get('/releases/latest'), get('')])
  return {
    version: typeof release?.tag_name === 'string' ? release.tag_name : '',
    stars: typeof repo?.stargazers_count === 'number' ? repo.stargazers_count : null,
  }
}

const { version, stars } = await githubFacts()

// Nav entries that only exist when their number was actually fetched, so
// the bar never shows a bare star or an empty version.
const releasesText = version ? `Releases ${version}` : 'Releases'
const starsNav =
  stars === null
    ? []
    : [
        {
          // Intl gives 1.2k rather than 1200 once the number grows.
          text: `★ ${new Intl.NumberFormat('en-GB', { notation: 'compact' }).format(stars)}`,
          link: `https://github.com/${REPO}/stargazers`,
        },
      ]

// The site is served from https://anas1412.github.io/ytmgo/, so every
// asset and link needs that prefix. Without `base` the built site
// looks fine locally and 404s on every stylesheet once deployed.
export default defineConfig({
  title: 'ytmgo',
  description: 'A terminal-based YouTube Music client. Search, download, queue, and play music from the keyboard.',
  base: '/ytmgo/',
  lang: 'en-GB',
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['link', { rel: 'icon', href: '/ytmgo/ytmgo-icon.png' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:title', content: 'ytmgo: YouTube Music from the terminal' }],
    ['meta', { property: 'og:description', content: 'Search, download, queue, and play music, all from the keyboard, inside your terminal.' }],
    ['meta', { property: 'og:image', content: 'https://raw.githubusercontent.com/anas1412/ytmgo/main/ytmgo.png' }],
    ['meta', { property: 'og:image:width', content: '1200' }],
    ['meta', { property: 'og:image:height', content: '631' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:image', content: 'https://raw.githubusercontent.com/anas1412/ytmgo/main/ytmgo.png' }],
  ],

  themeConfig: {
    logo: '/ytmgo-icon.png',

    nav: [
      { text: 'Guide', link: '/guide/install', activeMatch: '/guide/' },
      { text: 'Keybindings', link: '/guide/keybindings' },
      { text: 'CLI', link: '/guide/cli' },
      {
        text: releasesText,
        link: `https://github.com/${REPO}/releases`,
      },
      ...starsNav,
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Getting started',
          items: [
            { text: 'Install', link: '/guide/install' },
            { text: 'Keybindings', link: '/guide/keybindings' },
            { text: 'Command line', link: '/guide/cli' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/anas1412/ytmgo' },
    ],

    search: { provider: 'local' },

    editLink: {
      pattern: 'https://github.com/anas1412/ytmgo/edit/main/docs/:path',
      text: 'Edit this page on GitHub',
    },

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © anas1412',
    },
  },
})
