import type { HoveredImage } from './image-buttons';

export function isThreadsHost(hostname: string): boolean {
	return hostname === 'threads.com' || hostname.endsWith('.threads.com');
}

// Threads' photo tiles receive hover on <picture>, while video posters sit
// beside a <video> element. Only post media links are saveable here.
export function resolveThreadsImage(target: Element): HoveredImage | null {
	const postMedia = target.closest<HTMLAnchorElement>('a[href*="/post/"]');
	if (!postMedia || postMedia.querySelector('video')) return null;
	const surface = target.closest('picture') ?? postMedia;
	const image = surface.querySelector('img');
	return image && !image.parentElement?.querySelector('video') ? { image, surface } : null;
}
