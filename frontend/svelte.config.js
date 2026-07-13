import { mdsvex } from 'mdsvex';
import adapter from '@sveltejs/adapter-static';
import { relative, sep, join } from 'node:path';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		// defaults to rune mode for the project, except for `node_modules`. Can be removed in svelte 6.
		// mdsvex's generated wrapper code (for .svx/.md posts) still uses legacy `$$props`, so those
		// are left to auto-detect (-> legacy mode) rather than forced into runes mode.
		runes: ({ filename }) => {
			const relativePath = relative(import.meta.dirname, filename);
			const pathSegments = relativePath.toLowerCase().split(sep);
			const isExternalLibrary = pathSegments.includes('node_modules');
			const isMarkdownPost = /\.(svx|md)$/.test(filename);

			return isExternalLibrary || isMarkdownPost ? undefined : true;
		}
	},
	kit: {
		// SPA build: Capacitor loads static files from build/, with a catch-all fallback
		// so client-side routing works for every path.
		adapter: adapter({
			pages: 'build',
			assets: 'build',
			fallback: 'index.html',
			precompress: false,
			strict: false
		}),
		prerender: {
			handleHttpError: ({ path, status, message }) => {
				// Expected until the standard.site publication record is published and
				// STANDARD_SITE_PUBLICATION_RKEY is filled in — see frontend/src/lib/standard-site.ts.
				if (path === '/.well-known/site.standard.publication' && status === 404) return;
				throw new Error(message);
			},
			// The site.standard.document <link href="at://..."> in blog-post-layout.svelte is a
			// valid AT-URI but not a parseable WHATWG URL (colons in the did:plc: authority), so the
			// prerender crawler can't resolve it as a normal link — this is expected, not a broken link.
			handleInvalidUrl: 'ignore'
		}
	},
	preprocess: [
		mdsvex({
			extensions: ['.svx', '.md'],
			// mdsvex inserts this path verbatim as a relative import from each post's own
			// directory, so it must be absolute to resolve correctly regardless of nesting.
			layout: join(import.meta.dirname, 'src/lib/components/blog-post-layout.svelte')
		})
	],
	extensions: ['.svelte', '.svx', '.md']
};

export default config;
