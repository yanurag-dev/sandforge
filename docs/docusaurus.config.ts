import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Sandforge',
  tagline: 'Secure hypervisor-level isolation sandbox for AI coding agents',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
  },

  url: 'https://yanurag-dev.github.io',
  baseUrl: '/sandforge/',

  organizationName: 'yanurag-dev',
  projectName: 'sandforge',

  onBrokenLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    mermaid: true,
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },
  themes: ['@docusaurus/theme-mermaid'],
  plugins: [
    './src/plugins/tailwind-plugin.cjs', // Custom PostCSS bridge for Tailwind CSS
  ],

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          routeBasePath: 'docs', // Serve documentation under /docs/
        },
        blog: false, // Disables default blog template
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/docusaurus-social-card.jpg',
    colorMode: {
      defaultMode: 'dark',
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'Sandforge',
      logo: {
        alt: 'Sandforge Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          to: '/docs/api-reference',
          position: 'left',
          label: 'API',
        },
        {
          to: '/docs/cli',
          position: 'left',
          label: 'CLI',
        },
        {
          href: 'https://github.com/yanurag-dev/sandforge',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {
              label: 'Overview',
              to: '/docs/intro',
            },
            {
              label: 'Quickstart',
              to: '/docs/quickstart',
            },
            {
              label: 'Authentication',
              to: '/docs/authentication',
            },
          ],
        },
        {
          title: 'Architecture & Design',
          items: [
            {
              label: 'System Architecture',
              to: '/docs/architecture',
            },
            {
              label: 'Policy Engine Details',
              to: '/docs/policy-engine',
            },
            {
              label: 'macOS Virtualization',
              to: '/docs/macos-driver',
            },
          ],
        },
        {
          title: 'Development',
          items: [
            {
              label: 'Build and Test Guide',
              to: '/docs/development-guide',
            },
            {
              label: 'SDK Bindings',
              to: '/docs/sdks',
            },
            {
              label: 'Deployment Guide',
              to: '/docs/deployment',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Sandforge. Built with Docusaurus 3 + Tailwind CSS.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
