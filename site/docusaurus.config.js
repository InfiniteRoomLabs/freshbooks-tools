// @ts-check
// Docs-only Docusaurus config: renders docs/*.md + README.md (synced into
// site/docs/ by scripts/site-sync.sh, never hand-edited) as a static site.
// No blog, no search plugin, no external scripts/fonts/CDNs, no telemetry.

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'freshbooks-tools',
  tagline: 'A Go toolkit for the FreshBooks REST API',
  url: 'https://infiniteroomlabs.github.io',
  baseUrl: '/freshbooks-tools/',
  organizationName: 'InfiniteRoomLabs',
  projectName: 'freshbooks-tools',
  trailingSlash: false,

  onBrokenLinks: 'throw',

  // onBrokenMarkdownLinks used to be a top-level option; 3.10 deprecated
  // that spelling in favor of this nested one (still 'throw').
  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          routeBasePath: '/',
          sidebarPath: require.resolve('./sidebars.js'),
          // Per-page custom_edit_url front matter (set by site-sync.sh)
          // overrides this with the real docs/*.md or README.md source
          // path -- site/docs/*.md is generated and gitignored, so an
          // edit link built from this fallback would point nowhere real.
          editUrl: 'https://github.com/InfiniteRoomLabs/freshbooks-tools/edit/main/',
        },
        blog: false,
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
        // No gtag/googleAnalytics keys: D7, no telemetry.
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      colorMode: {
        // Not the framework default (false): follow the visitor's OS
        // preference, which is what D2's "dark mode on" rests on.
        respectPrefersColorScheme: true,
      },
      navbar: {
        title: 'freshbooks-tools',
        items: [
          {
            href: 'https://github.com/InfiniteRoomLabs/freshbooks-tools',
            label: 'GitHub',
            position: 'right',
          },
          {
            href: 'https://pkg.go.dev/github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks',
            label: 'pkg.go.dev',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Docs',
            items: [
              {label: 'Getting started', to: '/getting-started'},
              {label: 'CLI reference', to: '/cli'},
            ],
          },
          {
            title: 'More',
            items: [
              {label: 'GitHub', href: 'https://github.com/InfiniteRoomLabs/freshbooks-tools'},
              {label: 'Releases', href: 'https://github.com/InfiniteRoomLabs/freshbooks-tools/releases'},
            ],
          },
        ],
        copyright: `Copyright (c) ${new Date().getFullYear()} Infinite Room Labs. MIT licensed. Not affiliated with FreshBooks.`,
      },
      prism: {
        additionalLanguages: ['go', 'bash', 'yaml', 'json'],
      },
    }),
};

module.exports = config;
