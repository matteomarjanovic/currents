import { grainImageIdentity } from '../../lib/grain-origin';
import type { GrainImageOriginHint } from '../../lib/image-origin';

export function grainImageOrigin(img: HTMLImageElement): GrainImageOriginHint | undefined {
	if (location.hostname !== 'grain.social') return;
	const src = img.currentSrc || img.src;
	if (!src) return;
	const image = grainImageIdentity(src);
	if (!image) return;
	const galleries = new Set<string>();
	const add = (href: string) => {
		const url = new URL(href, location.href);
		const path = url.pathname.match(/^\/profile\/[^/]+\/gallery\/[^/]+(?=\/|$)/)?.[0];
		if (url.origin === 'https://grain.social' && path) galleries.add(url.origin + path);
	};
	const card = img.closest('article.gallery-card');
	if (!card) add(location.href);
	if (!galleries.size) {
		for (
			let node: HTMLElement | null = img;
			node && node !== document.body;
			node = node.parentElement
		) {
			if (node instanceof HTMLAnchorElement) add(node.href);
			for (const link of node.querySelectorAll<HTMLAnchorElement>('a[href*="/gallery/"]'))
				add(link.href);
			if (galleries.size || node === card) break;
		}
	}
	if (galleries.size) return { app: 'grain', ...image, galleries: [...galleries].slice(0, 4) };
}
