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

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          routeBasePath: '/', // Serve docs directly at base URL (ideal for documentation-only sites)
        },
        blog: false, // Disables blog section for a clean docs-only portal
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
          sidebarId: 'tutorialSidebar',
          position: 'left',
          label: 'Documentation',
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
              to: '/',
            },
          ],
        },
        {
          title: 'Architecture & Design',
          items: [
            {
              label: 'System Architecture',
              to: '/architecture',
            },
            {
              label: 'Policy Engine Details',
              to: '/policy-engine',
            },
          ],
        },
        {
          title: 'Development',
          items: [
            {
              label: 'macOS Virtualization',
              to: '/macos-driver',
            },
            {
              label: 'Build and Test Guide',
              to: '/development-guide',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Sandforge. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
