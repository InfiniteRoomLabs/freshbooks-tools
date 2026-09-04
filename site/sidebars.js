// Explicit order. This table is the only place the sidebar order lives;
// scripts/site-sync.sh writes no `sidebar_position` front matter. `index`
// (the synced README.md, the site home page) is deliberately not listed
// here -- it is a landing page, not part of the guide sequence.

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docsSidebar: [
    'getting-started',
    'authentication',
    'library',
    'mcp',
    'cli',
    'building',
    'agentic-transformation',
  ],
};

module.exports = sidebars;
