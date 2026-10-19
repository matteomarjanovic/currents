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
	const record = leafletDocumentUri(uri || canonical || '');
	if (record)
		return { app: 'leaflet', ...identity, uri: record, pages: [location.href.split('#')[0]] };
}
