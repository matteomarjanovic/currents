import { defineConfig } from 'wxt';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
	srcDir: 'src',
	modules: ['@wxt-dev/module-svelte'],
	alias: {
		$lib: 'src/lib'
	},
	vite: () => ({
		plugins: [tailwindcss()]
	}),
	runner: {
		binaries: {
			firefox: process.env.FIREFOX_BINARY
		}
	},
	manifest: ({ browser }) => ({
		name: 'Save to Currents',
		description: 'Save images to Currents',
		// No default_popup: clicking the toolbar icon opens the in-page picker for
		// every image on the page, which needs action.onClicked to fire.
		action: { default_title: 'Save images from this page' },
		permissions: ['contextMenus', 'activeTab', 'storage', 'cookies'],
		host_permissions: ['<all_urls>', 'https://currents.is/*'],
		web_accessible_resources: [
			{
				resources: ['fonts/*', 'icon/*'],
				matches: ['<all_urls>']
			}
		],
		...(browser === 'firefox' && {
			browser_specific_settings: {
				gecko: {
					id: 'extension@currents.is',
					strict_min_version: '142.0',
					data_collection_permissions: {
						required: ['none'],
						optional: []
					}
				}
			}
		})
	})
});
