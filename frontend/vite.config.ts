import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { SvelteKitPWA } from '@vite-pwa/sveltekit';
import { defineConfig } from 'vite';
import browserslist from 'browserslist';
import { browserslistToTargets } from 'lightningcss';

// The static build is reused by Capacitor (capacitor.config.ts webDir: 'build'). A
// service worker is unwanted inside the native webview (it causes stale-cache bugs),
// so the PWA plugin runs only for the web build — the mobile build scripts set
// CAPACITOR=1 to skip it.
const isCapacitor = !!process.env.CAPACITOR;

export default defineConfig(({ command }) => ({
	plugins: [
		tailwindcss(),
		sveltekit(),
		...(isCapacitor
			? []
			: [
					SvelteKitPWA({
						registerType: 'autoUpdate',
						// Registration is done manually in src/routes/+layout.svelte (web-only,
						// guarded by isNative()), so don't emit/inject the registerSW helper.
						injectRegister: false,
						manifest: {
							name: 'Currents: Save & Discover',
							short_name: 'Currents',
							description: 'Save and curate images into collections. Discover more. Own your data.',
							theme_color: '#ffffff',
							background_color: '#ffffff',
							display: 'standalone',
							orientation: 'portrait',
							scope: '/',
							start_url: '/',
							icons: [
								{ src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
								{ src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
								{
									src: '/icons/icon-maskable-192.png',
									sizes: '192x192',
									type: 'image/png',
									purpose: 'maskable'
								},
								{
									src: '/icons/icon-maskable-512.png',
									sizes: '512x512',
									type: 'image/png',
									purpose: 'maskable'
								}
							],
							// Receive images/links shared to the installed PWA from the OS share sheet.
							// Files require POST/multipart; the static site has no server, so the service
							// worker intercepts this POST (see static/share-target-sw.js).
							share_target: {
								action: '/share-target',
								method: 'POST',
								enctype: 'multipart/form-data',
								params: {
									title: 'title',
									text: 'text',
									url: 'url',
									files: [{ name: 'image', accept: ['image/*'] }]
								}
							}
						},
						workbox: {
							globPatterns: ['client/**/*.{js,css,ico,png,svg,webp,woff,woff2}'],
							// Layer our Web Share Target POST handler onto the generated Workbox SW.
							importScripts: ['/share-target-sw.js']
						},
						// Serve the manifest + a dev service worker under `vite dev` too, so the PWA
						// (and the /manifest.webmanifest link in app.html) works without a full build.
						devOptions: {
							enabled: true,
							type: 'module',
							suppressWarnings: true
						}
					})
				])
	],
	ssr: { noExternal: ['@masonry-grid/svelte', '@masonry-grid/core'] },
	// Tailwind v4 emits oklch() throughout (its whole palette + our theme tokens), which
	// Chromium <111 (e.g. Brave from 2022) can't parse and drops, breaking the styling. Run
	// the CSS through Lightning CSS targeting older browsers so it adds a hex fallback ahead
	// of each modern color (progressive enhancement — modern browsers still use oklch/lab).
	// Build-only: dev is always a modern browser, and the dev pipeline doesn't preserve the
	// dual declaration, so running it there would needlessly downgrade local colors.
	// (color-mix() with var() args — a few cosmetic washes in layout.css — can't be statically
	// downleveled and degrade to a flat background on old browsers.)
	...(command === 'build'
		? {
				css: {
					transformer: 'lightningcss' as const,
					lightningcss: {
						targets: browserslistToTargets(
							browserslist('Chrome >= 99, Firefox >= 99, Safari >= 15, iOS >= 15')
						)
					}
				},
				build: { cssMinify: 'lightningcss' as const }
			}
		: {})
}));
