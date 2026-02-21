/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docs: [
    'intro',
    'architecture',
    'configuration',
    {
      type: 'category',
      label: 'Security',
      items: [
        'authentication',
        'authorization',
        'credential-management',
      ],
    },
    'transports',
    'session-management',
    'kubernetes-operator',
    'observability',
    'api-reference',
  ],
};

module.exports = sidebars;
