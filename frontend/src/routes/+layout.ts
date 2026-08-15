import { browser, building } from '$app/environment';
import { apiFetch } from '$lib/api';
import type { LayoutLoad } from './$types';

// The web build renders on the server. Capacitor has no server, so its static artifact retains
// the existing SPA behavior and hydrates entirely in the webview.
export const ssr = !import.meta.env.VITE_CAPACITOR;
export const prerender = false;
export const trailingSlash = 'ignore';

export const load: LayoutLoad = async ({ fetch }) => {
	// Browser auth is refreshed by the application layouts after mount. This server pass lets the
	// final same-origin deployment render a known viewer immediately without making Capacitor
	// depend on a server-only load or calling production APIs while prerendering marketing pages.
	if (browser || building) return { viewer: null };

	try {
		const res = await apiFetch('/api/me', {}, fetch);
		if (res.ok) return { viewer: await res.json() };
	} catch {
		// Public pages still render when appview is unavailable.
	}
	return { viewer: null };
};
