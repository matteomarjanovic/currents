import { leafletDocumentUri, leafletImageIdentity } from '../../lib/leaflet-origin';
import type { LeafletImageOriginHint } from '../../lib/image-origin';

export function leafletImageOrigin(img: HTMLImageElement): LeafletImageOriginHint | undefined {
	const identity = leafletImageIdentity(img.currentSrc || img.src);
	if (!identity) return;
	const canonical = document.querySelector<HTMLMetaElement>('meta[name="at:canonical"]')?.content;
	const uri = document.querySelector<HTMLLinkElement>('link[rel="site.standard.document"]')?.href;
	if (
		location.hostname !== 'leaflet.pub' &&
		!location.hostname.endsWith('.leaflet.pub') &&
		!canonical?.match(
			/^at:\/\/did:[^/]+\/(?:site\.standard|pub\.leaflet)\.(?:document|publication)\//
		) &&
		!(uri && leafletDocumentUri(uri))
	)
		return;
	// Feed and embedded-post cards have a sibling overlay link, not an enclosing anchor.
	const card = img.closest('.postItemMedium, .postLinkLarge');
	const link = card?.querySelector<HTMLAnchorElement>(':scope > a.publishedPostLink[href]');
	if (card) return link ? { app: 'leaflet', ...identity, pages: [link.href] } : undefined;
	const reader = img.closest('.readerLeafletPost');
	if (reader) {
		// The reader keeps the feed URL/head, but the article's main page ID
		// carries its record URI. Subpages use their own IDs beside that main page.
		const page =
			img.closest('[id^="post-page-at://"]') ?? reader.querySelector('[id^="post-page-at://"]');
		const record = leafletDocumentUri(page?.id.slice('post-page-'.length) || '');
		if (!record) return;
		const [did, , rkey] = record.slice(5).split('/');
		return {
			app: 'leaflet',
			...identity,
			uri: record,
			pages: [`https://leaflet.pub/p/${encodeURIComponent(did)}/${rkey}`]
		};
	}
	const record = leafletDocumentUri(uri || canonical || '');
	if (record)
		return { app: 'leaflet', ...identity, uri: record, pages: [location.href.split('#')[0]] };
}
