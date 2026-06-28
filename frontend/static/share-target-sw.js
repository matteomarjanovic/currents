// Web Share Target handler, layered onto the generated Workbox service worker via
// vite.config.ts (workbox.importScripts). The manifest's share_target makes the browser
// POST multipart/form-data to /share-target when the user shares to the installed PWA.
// Since the site is statically hosted (no server), the service worker handles that POST
// here: it stashes any shared image in a cache and redirects to the SPA route
// /share-target, which turns it into share.pending and forwards to /upload.
//
// Keep SHARE_CACHE / SHARE_KEY in sync with src/lib/share-target.ts.
const SHARE_CACHE = 'share-target';
const SHARE_KEY = '/__shared-image';

self.addEventListener('fetch', (event) => {
	const url = new URL(event.request.url);
	if (event.request.method !== 'POST' || url.pathname !== '/share-target') return;

	// Absolute URLs — Response.redirect() rejects relative ones in some engines.
	const redirect = (path) => Response.redirect(url.origin + path, 303);

	event.respondWith(
		(async () => {
			try {
				const form = await event.request.formData();
				const file = form.get('image');
				if (file && typeof file !== 'string' && file.size > 0) {
					const cache = await caches.open(SHARE_CACHE);
					await cache.put(
						SHARE_KEY,
						new Response(file, {
							headers: {
								'content-type': file.type || 'application/octet-stream',
								'x-filename': file.name || ''
							}
						})
					);
					return redirect('/share-target?shared=image');
				}
				// No file — a shared link/text. Forward the fields for the SPA route to parse.
				const params = new URLSearchParams({ shared: 'url' });
				for (const key of ['url', 'text', 'title']) {
					const value = form.get(key);
					if (typeof value === 'string' && value) params.set(key, value);
				}
				return redirect('/share-target?' + params.toString());
			} catch {
				return redirect('/upload');
			}
		})()
	);
});
