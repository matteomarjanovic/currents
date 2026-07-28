import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { SvelteKitPWA } from '@vite-pwa/sveltekit';
import { defineConfig } from 'vite';

// The static build is reused by Capacitor (capacitor.config.ts webDir: 'build'). A
// service worker is unwanted inside the native webview (it causes stale-cache bugs),
// so the PWA plugin runs only for the web build — the mobile build scripts set
// CAPACITOR=1 to skip it.
const isCapacitor = !!process.env.CAPACITOR;

export default defineConfig({
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
							// NOTE: `action` must be an ABSOLUTE URL — Android's WebAPK minter silently
							// fails to register the share intent-filter for a relative action path.
							share_target: {
								action: 'https://currents.is/share-target',
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
							// @vite-pwa/sveltekit appends `prerendered/**/*.{html,json}` to globPatterns
							// on its own, which pulls the prerendered pages (the blog, /support-us) into
							// the precache — and precache entries are served CACHE-FIRST. A returning
							// visitor would then keep seeing the copy from whatever deploy installed their
							// worker, so a newly published post only appeared after a hard refresh (which
							// mobile browsers don't offer). Netlify serves HTML with `must-revalidate`,
							// so letting these hit the network costs one conditional request and is always
							// current. Content pages don't belong in a precache.
							globIgnores: ['prerendered/**'],
							// `registerType: 'autoUpdate'` only drives the virtual registerSW module, which
							// this app doesn't use (injectRegister: false — registration is manual in
							// +layout.svelte). Without these two, the generated worker calls skipWaiting()
							// only on a `SKIP_WAITING` message that nothing ever posts, so an updated worker
							// sits in "waiting" forever: the precache never refreshes and cleanupOutdatedCaches
							// never runs until every tab on the origin is closed.
							clientsClaim: true,
							skipWaiting: true,
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
	build: {
		// @capawesome-team/* ships from a paid private registry (Insiders licence). It's declared
		// as an optionalDependency, so an install without a valid CAPAWESOME_TOKEN just warns and
		// skips it instead of failing the deploy — which means it may or may not be on disk when
		// the web build runs. Only the native builds actually need it: every call site in
		// auth-storage.ts sits behind an `isNative()` guard, so in a web bundle the dynamic import
		// is dead code. Externalizing it makes the web build independent of whether it installed.
		rollupOptions: {
			external: isCapacitor ? [] : ['@capawesome-team/capacitor-secure-preferences']
		}
	},
	ssr: { noExternal: ['@masonry-grid/svelte', '@masonry-grid/core'] }
});
