import { defineConfig } from 'vitepress'

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
        text: 'Releases',
        link: 'https://github.com/anas1412/ytmgo/releases',
      },
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
