import { isOffprintUrl, offprintPageOrigin } from '../../lib/offprint-origin';
import type { OffprintImageOriginHint } from '../../lib/image-origin';

export function offprintImageOrigin(img: HTMLImageElement): OffprintImageOriginHint | undefined {
	if (!isOffprintUrl(location.href)) return;
	const imageUrl = img.currentSrc || img.src;
	if (!/^https?:/.test(imageUrl)) return;
	try {
		const page =
			history.state?.page ??
			JSON.parse(document.querySelector('script[data-page="app"]')?.textContent || 'null');
		const snapshot = offprintPageOrigin(page, imageUrl, location.href);
		if (snapshot) return { app: 'offprint', imageUrl, pages: [snapshot.originUrl], snapshot };
	} catch {
		/* The enclosing article link can still resolve through its public page. */
	}
	const pages = new Set<string>();
	for (
		let node: HTMLElement | null = img;
		node && node !== document.body;
		node = node.parentElement
	) {
		const links =
			node instanceof HTMLAnchorElement
				? [node]
				: node.querySelectorAll<HTMLAnchorElement>('a[href]');
		for (const link of links) {
			if (isOffprintUrl(link.href) && /^\/a\/[^/]+\/?$/.test(new URL(link.href).pathname))
				pages.add(link.href.split('#')[0]);
		}
		if (pages.size) break;
	}
	if (pages.size) return { app: 'offprint', imageUrl, pages: [...pages].slice(0, 4) };
}
