// @ts-check

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Reflow Gateway',
  tagline: 'MCP multiplexing gateway with authentication, authorization, and credential injection',
  favicon: 'img/favicon.svg',

  url: 'https://reflowgateway.com',
  baseUrl: '/docs/',
  organizationName: 'JulianPedro',
  projectName: 'reflow-gateway',

  onBrokenLinks: 'throw',

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.js',
          editUrl: 'https://github.com/JulianPedro/reflow-gateway/tree/main/website/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      navbar: {
        title: 'Reflow Gateway',
        logo: {
          alt: 'Reflow Gateway Logo',
          src: 'img/favicon.svg',
        },
        items: [
          {
            href: 'https://github.com/JulianPedro/reflow-gateway',
            label: 'GitHub',
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
              { label: 'Getting Started', to: '/' },
              { label: 'Architecture', to: '/architecture' },
              { label: 'API Reference', to: '/api-reference' },
            ],
          },
          {
            title: 'Community',
            items: [
              { label: 'GitHub', href: 'https://github.com/JulianPedro/reflow-gateway' },
              { label: 'Issues', href: 'https://github.com/JulianPedro/reflow-gateway/issues' },
            ],
          },
        ],
        copyright: `Copyright ${new Date().getFullYear()} Reflow Gateway. Built with Docusaurus.`,
      },
      prism: {
        theme: require('prism-react-renderer').themes.github,
        darkTheme: require('prism-react-renderer').themes.dracula,
        additionalLanguages: ['bash', 'json', 'yaml', 'go'],
      },
    }),
};

module.exports = config;
