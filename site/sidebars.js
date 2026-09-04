// Explicit order (mirrors each doc's front matter `sidebar_position`, set
// by scripts/site-sync.sh). `index` (the synced README.md, the site home
// page) is deliberately not listed here -- it is a landing page, not part
// of the guide sequence.

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
