import type { HoveredImage } from './image-buttons';

export function isInstagramHost(hostname: string): boolean {
	return hostname === 'instagram.com' || hostname.endsWith('.instagram.com');
}

function photoIn(surface: Element): HoveredImage | null {
	const image = surface.querySelector('img');
	if (
		!image ||
		/^Video by /i.test(image.alt) ||
		surface.querySelector(
			'video,svg[aria-label="Clip"],svg[aria-label="Video"],svg[aria-label="Reel"]'
		)
	)
		return null;
	return { image, surface };
}

// Instagram puts transparent overlays above both grid thumbnails and post
// media, so hovering those overlays never targets the underlying <img>.
export function resolveInstagramImage(target: Element): HoveredImage | null {
	const post = target.closest<HTMLAnchorElement>('a[href*="/p/"],a[href*="/reel/"]');
	if (post) return post.pathname.match(/\/p\/[^/]+\/?$/) ? photoIn(post) : null;

	if (/\/reel\//.test(location.pathname)) return null;
	const media = target.closest('._aagu');
	if (media) return photoIn(media);
	if (!(target instanceof HTMLImageElement)) return null;
	if (/^Video by /i.test(target.alt) || target.closest('article')?.querySelector('video'))
		return null;
	return { image: target, surface: target };
}
