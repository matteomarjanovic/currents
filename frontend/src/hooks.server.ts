import { env } from '$env/dynamic/private';
import { PUBLIC_APPVIEW_URL, PUBLIC_WEB_APPVIEW_URL } from '$env/static/public';
import type { Handle, HandleFetch } from '@sveltejs/kit';

// app.html's og:image/twitter:image are a site-wide default that can't be templated per-route;
// strip them when an SSR/prerendered page provides its own so crawlers do not see conflicts.
const DEFAULT_OG_IMAGE = '/currents_og_image.png';
const DEFAULT_OG_IMAGE_PATTERN = DEFAULT_OG_IMAGE.replace(/\./g, '\\.');

const DEFAULT_OG_IMAGE_TAG = new RegExp(
	`\\s*<meta property="og:image" content="${DEFAULT_OG_IMAGE_PATTERN}"\\s*/?>`
);
const DEFAULT_TWITTER_IMAGE_TAG = new RegExp(
	`\\s*<meta name="twitter:image" content="${DEFAULT_OG_IMAGE_PATTERN}"\\s*/?>`
);

export const handle: Handle = async ({ event, resolve }) => {
	return resolve(event, {
		transformPageChunk: ({ html }) => {
			const ogImages = [...html.matchAll(/<meta property="og:image" content="([^"]*)"/g)];
			const hasOverride = ogImages.some(([, content]) => !content.endsWith(DEFAULT_OG_IMAGE));
			if (!hasOverride) return html;

			return html.replace(DEFAULT_OG_IMAGE_TAG, '').replace(DEFAULT_TWITTER_IMAGE_TAG, '');
		}
	});
};

const APPVIEW_PATHS = ['/api/', '/xrpc/', '/oauth/', '/img/'];

// Browser requests are routed by Caddy and never pass through SvelteKit. Server-side loads use
// this hook to reach appview over the Compose network and carry the incoming host-only cookie.
export const handleFetch: HandleFetch = async ({ event, request, fetch }) => {
	const internalBase = env.APPVIEW_INTERNAL_URL?.replace(/\/$/, '');
	if (!internalBase) return fetch(request);

	const url = new URL(request.url);
	const publicAppviewOrigins = [PUBLIC_APPVIEW_URL, PUBLIC_WEB_APPVIEW_URL]
		.filter(Boolean)
		.map((value) => new URL(value).origin);
	const isAppviewRequest =
		APPVIEW_PATHS.some((prefix) => url.pathname.startsWith(prefix)) &&
		(url.origin === event.url.origin || publicAppviewOrigins.includes(url.origin));
	if (!isAppviewRequest) return fetch(request);

	const proxied = new Request(`${internalBase}${url.pathname}${url.search}`, request);
	const cookie = event.request.headers.get('cookie');
	if (cookie) proxied.headers.set('cookie', cookie);

	return fetch(proxied);
};
