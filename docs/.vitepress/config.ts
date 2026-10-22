import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'ytmgo',
  description: 'A free YouTube Music client with no account and no API key: a desktop app for Windows, a terminal app for Linux.',
  base: '/ytmgo/',
  lang: 'en-GB',
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['link', { rel: 'icon', href: '/ytmgo/ytmgo-icon.png' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:title', content: 'ytmgo: YouTube Music without the browser' }],
    ['meta', { property: 'og:description', content: 'A free YouTube Music client with no account and no API key. A desktop app for Windows, a terminal app for Linux.' }],
    ['meta', { property: 'og:image', content: 'https://anas1412.github.io/ytmgo/og.jpg' }],
    ['meta', { property: 'og:image:width', content: '1200' }],
    ['meta', { property: 'og:image:height', content: '630' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:image', content: 'https://anas1412.github.io/ytmgo/og.jpg' }],
  ],

  themeConfig: {
    logo: '/ytmgo-icon.png',

    nav: [
      { text: 'Guide', link: '/guide/install', activeMatch: '/guide/' },
      { text: 'Keybindings', link: '/guide/keybindings' },
      { text: 'CLI', link: '/guide/cli' },
      {
        // The version is appended client-side; see the theme.
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
            { text: 'Browsing', link: '/guide/browsing' },
            { text: 'Keybindings', link: '/guide/keybindings' },
            { text: 'Command line', link: '/guide/cli' },
            { text: 'Troubleshooting', link: '/guide/troubleshooting' },
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
