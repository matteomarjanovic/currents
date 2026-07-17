#!/usr/bin/env node
// Generates the Open Graph image for each blog post at build time (see the `prebuild` npm
// script) — the blog is prerendered specifically for SEO, so its preview images should be too,
// rather than depending on a runtime image-generation endpoint.
//
// Base template (logo + photo grid) supplied as a flat PNG; this script only lays out the
// "Into the currents" kicker + post title on top of it via satori (JSX-like tree -> SVG),
// rasterized with resvg. Output goes to static/og/<slug>.png, which SITE_URL + /og/<slug>.png
// is what blog-post-layout.svelte points og:image at.

import { readFileSync, readdirSync, mkdirSync, writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import satori from 'satori';
import { Resvg } from '@resvg/resvg-js';
import matter from 'gray-matter';

const ROOT = dirname(fileURLToPath(import.meta.url));
const BLOG_DIR = join(ROOT, '../src/routes/blog');
const OUT_DIR = join(ROOT, '../static/og');
const TEMPLATE_PATH = join(ROOT, 'assets/og-template.png');

const WIDTH = 1200;
const HEIGHT = 630;

const fontRegular = readFileSync(
	join(
		ROOT,
		'../node_modules/@fontsource/instrument-sans/files/instrument-sans-latin-600-normal.woff'
	)
);
const fontBold = readFileSync(
	join(
		ROOT,
		'../node_modules/@fontsource/instrument-sans/files/instrument-sans-latin-700-normal.woff'
	)
);
const templateBase64 = readFileSync(TEMPLATE_PATH).toString('base64');

function findPosts() {
	return readdirSync(BLOG_DIR, { withFileTypes: true })
		.filter((e) => e.isDirectory())
		.flatMap((dir) => {
			const svx = ['+page.svx', '+page.md']
				.map((f) => join(BLOG_DIR, dir.name, f))
				.find((p) => {
					try {
						readFileSync(p);
						return true;
					} catch {
						return false;
					}
				});
			if (!svx) return [];
			const { data } = matter(readFileSync(svx, 'utf-8'));
			return [{ slug: dir.name, title: data.title }];
		});
}

async function renderOgImage(title) {
	const svg = await satori(
		{
			type: 'div',
			props: {
				style: {
					width: `${WIDTH}px`,
					height: `${HEIGHT}px`,
					display: 'flex',
					backgroundImage: `url(data:image/png;base64,${templateBase64})`
				},
				children: {
					type: 'div',
					props: {
						style: {
							position: 'absolute',
							left: '64px',
							bottom: '68px',
							width: '650px',
							display: 'flex',
							flexDirection: 'column',
							gap: '14px'
						},
						children: [
							{
								type: 'div',
								props: {
									style: {
										display: 'flex',
										fontFamily: 'Instrument Sans',
										fontWeight: 600,
										fontSize: '24px',
										color: 'rgba(255, 255, 255, 0.65)'
									},
									children: 'Into the currents'
								}
							},
							{
								type: 'div',
								props: {
									style: {
										display: 'flex',
										fontFamily: 'Instrument Sans',
										fontWeight: 700,
										fontSize: '46px',
										lineHeight: 1.15,
										color: '#ffffff'
									},
									children: title
								}
							}
						]
					}
				}
			}
		},
		{
			width: WIDTH,
			height: HEIGHT,
			fonts: [
				{ name: 'Instrument Sans', data: fontRegular, weight: 600, style: 'normal' },
				{ name: 'Instrument Sans', data: fontBold, weight: 700, style: 'normal' }
			]
		}
	);

	return new Resvg(svg, { fitTo: { mode: 'width', value: WIDTH } }).render().asPng();
}

mkdirSync(OUT_DIR, { recursive: true });

const posts = findPosts();
for (const post of posts) {
	const png = await renderOgImage(post.title);
	writeFileSync(join(OUT_DIR, `${post.slug}.png`), png);
	console.log(`og image: ${post.slug}.png`);
}
console.log(`Generated ${posts.length} OG image(s).`);
