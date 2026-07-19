import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */
const sidebars: SidebarsConfig = {
  docsSidebar: [
    'home',
    {
      type: 'category',
      label: 'Foundations',
      items: [
        'project-overview',
        'getting-started',
        'prerequisites',
        'installation',
        'configuration',
        'first-working-example',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: ['tutorials', 'task-oriented-guides'],
    },
    {
      type: 'category',
      label: 'Reference',
      items: ['architecture-concepts', 'api-command-reference'],
    },
    'troubleshooting',
    'faq',
    'contributing-docs',
  ],
};

export default sidebars;
