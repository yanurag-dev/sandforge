import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'intro',
        'installation',
        'quickstart',
        'authentication',
      ],
    },
    {
      type: 'category',
      label: 'Core Reference',
      collapsed: false,
      items: [
        'api-reference',
        'cli',
        'webhooks',
        'sdks',
      ],
    },
    {
      type: 'category',
      label: 'Architecture & Design',
      collapsed: true,
      items: [
        'architecture',
        'policy-engine',
        'macos-driver',
        'guides',
      ],
    },
    {
      type: 'category',
      label: 'Resources',
      collapsed: true,
      items: [
        'development-guide',
        'tutorials',
        'deployment',
        'troubleshooting',
        'faq',
      ],
    },
  ],
};

export default sidebars;
