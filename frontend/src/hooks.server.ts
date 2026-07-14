// This only ever runs during prerendering (blog + support-us) — the rest of the app is a static
// SPA with no server at runtime. app.html's og:image/twitter:image are a site-wide default that
// can't be templated per-route; strip them here when a page has set its own (e.g. blog posts, via
// blog-post-layout.svelte), so crawlers don't see two conflicting og:image tags.
const DEFAULT_OG_IMAGE = '/currents_og_image.png';
const DEFAULT_OG_IMAGE_PATTERN = DEFAULT_OG_IMAGE.replace(/\./g, '\\.');

const DEFAULT_OG_IMAGE_TAG = new RegExp(
	`\\s*<meta property="og:image" content="${DEFAULT_OG_IMAGE_PATTERN}"\\s*/?>`
);
const DEFAULT_TWITTER_IMAGE_TAG = new RegExp(
	`\\s*<meta name="twitter:image" content="${DEFAULT_OG_IMAGE_PATTERN}"\\s*/?>`
);

export async function handle({ event, resolve }) {
	return resolve(event, {
		transformPageChunk: ({ html }) => {
			const ogImages = [...html.matchAll(/<meta property="og:image" content="([^"]*)"/g)];
			const hasOverride = ogImages.some(([, content]) => !content.endsWith(DEFAULT_OG_IMAGE));
			if (!hasOverride) return html;

			return html.replace(DEFAULT_OG_IMAGE_TAG, '').replace(DEFAULT_TWITTER_IMAGE_TAG, '');
		}
	});
}
