import { defineConfig } from 'wxt';

export default defineConfig({
  srcDir: 'src',
  modules: ['@wxt-dev/module-svelte'],
  manifest: {
    name: 'Currents Clipper',
    description: 'Save images to Currents',
    permissions: ['contextMenus', 'activeTab', 'storage', 'cookies'],
    host_permissions: ['<all_urls>'],
  },
});
