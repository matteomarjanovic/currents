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
		})
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
