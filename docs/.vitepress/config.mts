import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'open-warp',
  description: 'Use your own OpenAI-compatible LLM provider inside Warp.',
  base: '/open-warp/',
  cleanUrls: true,
  head: [
    ['meta', { name: 'theme-color', content: '#111827' }],
    ['meta', { property: 'og:title', content: 'open-warp' }],
    ['meta', { property: 'og:description', content: 'A local open-source AI backend for Warp.' }]
  ],
  themeConfig: {
    logo: '/logo.svg',
    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'Tools', link: '/guide/supported-tools' },
      { text: '中文', link: '/zh/' }
    ],
    sidebar: {
      '/guide/': [
        {
          text: 'Guide',
          items: [
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Configuration', link: '/guide/configuration' },
            { text: 'Warp Client', link: '/guide/warp-client' },
            { text: 'Supported Tools', link: '/guide/supported-tools' },
            { text: 'Troubleshooting', link: '/guide/troubleshooting' }
          ]
        }
      ],
      '/zh/guide/': [
        {
          text: '指南',
          items: [
            { text: '快速开始', link: '/zh/guide/getting-started' },
            { text: '配置说明', link: '/zh/guide/configuration' },
            { text: 'Warp 客户端', link: '/zh/guide/warp-client' },
            { text: '已支持工具', link: '/zh/guide/supported-tools' },
            { text: '故障排查', link: '/zh/guide/troubleshooting' }
          ]
        }
      ]
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/sasuke39/open-warp' }
    ],
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2026 open-warp contributors'
    },
    search: {
      provider: 'local'
    }
  }
})
