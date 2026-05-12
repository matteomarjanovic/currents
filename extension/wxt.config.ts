import { defineConfig } from 'wxt';

export default defineConfig({
  srcDir: 'src',
  modules: ['@wxt-dev/module-svelte'],
  runner: {
    binaries: {
      firefox: process.env.FIREFOX_BINARY,
    },
  },
  manifest: ({ browser }) => ({
    name: 'Save to Currents',
    description: 'Save images to Currents',
    permissions: ['contextMenus', 'activeTab', 'storage', 'cookies'],
    host_permissions: ['<all_urls>'],
    ...(browser === 'firefox' && {
      browser_specific_settings: {
        gecko: {
          id: 'extension@currents.is',
          strict_min_version: '140.0',
          data_collection_permissions: {
            required: ['none'],
            optional: [],
          },
        },
      },
    }),
  }),
});
