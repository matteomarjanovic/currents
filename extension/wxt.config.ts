import { defineConfig } from 'wxt';

export default defineConfig({
  srcDir: 'src',
  modules: ['@wxt-dev/module-svelte'],
  manifest: {
    name: 'Save to Currents',
    description: 'Save images to Currents',
    permissions: ['contextMenus', 'activeTab', 'storage', 'cookies'],
    host_permissions: ['<all_urls>'],
  },
});
