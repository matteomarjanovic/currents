import { isPcktUrl, pcktPageOrigin } from '../../lib/pckt-origin';
import type { PcktImageOriginHint } from '../../lib/image-origin';

export function pcktImageOrigin(img: HTMLImageElement): PcktImageOriginHint | undefined {
	if (!isPcktUrl(location.href)) return;
	if (img.closest('[data-non-content="true"]')) return;
	const imageUrl = img.currentSrc || img.src;
	if (!/^https?:/.test(imageUrl)) return;
	// Inertia updates history's page object during navigation; its initial JSON
	// script remains mounted and must not override the current page's metadata.
	try {
		const page =
			history.state?.page ??
			JSON.parse(document.querySelector('script[data-page="app"]')?.textContent || 'null');
		const snapshot = pcktPageOrigin(page, imageUrl, location.href);
		if (snapshot) return { app: 'pckt', imageUrl, pages: [snapshot.originUrl], snapshot };
	} catch {
		/* No usable page snapshot; try the enclosing post's public URL. */
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
			if (isPcktUrl(link.href) && new URL(link.href).pathname !== '/')
				pages.add(link.href.split('#')[0]);
		}
		if (pages.size) break;
	}
	if (pages.size) return { app: 'pckt', imageUrl, pages: [...pages].slice(0, 4) };
}
