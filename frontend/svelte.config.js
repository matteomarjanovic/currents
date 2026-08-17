import { mdsvex } from 'mdsvex';
import netlifyAdapter from '@sveltejs/adapter-netlify';
import nodeAdapter from '@sveltejs/adapter-node';
import staticAdapter from '@sveltejs/adapter-static';
import { relative, sep, join } from 'node:path';

// Keep in sync with the CAPACITOR guard in vite.config.ts and webDir in capacitor.config.ts.
const isCapacitor = !!process.env.CAPACITOR;
const isNetlify = process.env.DEPLOY_TARGET === 'netlify';

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
		// The web and native artifacts are deliberately different. Netlify packages SSR into a
		// function, Scaleway runs a Node server, and Capacitor keeps its static SPA fallback.
		adapter: isCapacitor
			? staticAdapter({
					pages: 'build-mobile',
					assets: 'build-mobile',
					fallback: 'index.html',
					precompress: false,
					strict: false
				})
			: isNetlify
				? netlifyAdapter()
				: nodeAdapter({ out: 'build' }),
		prerender: {
			handleHttpError: ({ path, status, message }) => {
				// Expected until the standard.site publication record is published and
				// STANDARD_SITE_PUBLICATION_RKEY is filled in — see frontend/src/lib/standard-site.ts.
				if (path === '/.well-known/site.standard.publication' && status === 404) return;
				// The PWA plugin (which emits /manifest.webmanifest, linked from app.html) is skipped
				// in the Capacitor build — see the CAPACITOR guard in vite.config.ts — so the crawler
				// can't resolve the manifest link there. The native webview never loads it anyway.
				if (path === '/manifest.webmanifest' && status === 404) return;
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
