import {
	externalSourceURL,
	isPinterestHost,
	pinterestPinUrl,
	samePinterestPin
} from '../../lib/pinterest-source';

export function pinterestImageSource(img: HTMLImageElement) {
	if (!isPinterestHost(location.hostname)) return;
	const card = img.closest('[data-test-id="pin"]');
	const id = card?.getAttribute('data-test-pin-id');
	const link = card?.querySelector<HTMLAnchorElement>('a[href*="/pin/"]');
	const hero = img.closest(
		'[data-test-id="closeup-body-image-container"], [data-test-id="closeup-image"], [data-test-id="pin-closeup-image"]'
	);
	const pin =
		id && /^\d+$/.test(id)
			? `${location.origin}/pin/${id}/`
			: link
				? pinterestPinUrl(link.href)
				: hero
					? pinterestPinUrl(location.href)
					: undefined;
	if (!pin) return;
	const source = card?.querySelector<HTMLAnchorElement>(
		'a[data-test-id="pinrep-source-link"], a[data-test-id="clickthrough-link"]'
	);
	let originUrl = externalSourceURL(source?.href);
	if (!originUrl && hero && samePinterestPin(pin, location.href)) {
		const clickthrough = document.querySelector<HTMLAnchorElement>(
			'a[data-test-id="clickthrough-link"]'
		);
		originUrl = externalSourceURL(clickthrough?.href);
		const canonical = document.querySelector<HTMLMetaElement>('meta[property="og:url"]')?.content;
		if (!originUrl && (!canonical || samePinterestPin(canonical, pin)))
			originUrl = externalSourceURL(
				document.querySelector<HTMLMetaElement>('meta[property="og:see_also"]')?.content
			);
	}
	return { originUrl: originUrl ?? pin, pinterestPinUrl: pin };
}
