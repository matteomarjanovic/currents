import type { ImageOrigin, OffprintImageOriginHint } from './image-origin';

export function isOffprintUrl(raw: string) {
	const url = new URL(raw);
	return (
		url.protocol === 'https:' &&
		(url.hostname === 'offprint.app' || url.hostname.endsWith('.offprint.app'))
	);
}

function imageKey(raw: string) {
	const url = new URL(raw);
	if (url.hostname === 'cdn.offprint.app') {
		const media = url.pathname.match(/^\/p\/\d+\//)?.[0];
		if (media) return url.origin + media;
	}
	return url.href;
}

interface Media {
	alt_text?: string;
	[key: string]: unknown;
}

function findMedia(media: unknown, key: string): Media | undefined {
	if (!media || typeof media !== 'object') return;
	if (Array.isArray(media)) {
		for (const item of media) {
			const match = findMedia(item, key);
			if (match) return match;
		}
		return;
	}
	const data = media as Media;
	return [
		'url',
		'thumbnail',
		'medium',
		'large',
		'original',
		'thumbnail_url',
		'medium_url',
		'large_url'
	].some((field) => {
		const src = data[field];
		return typeof src === 'string' && /^https?:/.test(src) && imageKey(src) === key;
	})
		? data
		: undefined;
}

function findContentImage(value: unknown, key: string): { alt?: string } | undefined {
	if (!value || typeof value !== 'object') return;
	if (Array.isArray(value)) {
		for (const item of value) {
			const image = findContentImage(item, key);
			if (image) return image;
		}
		return;
	}
	const block = value as {
		type?: string;
		attrs?: { mediaData?: unknown; src?: string; alt?: string; altText?: string };
		content?: unknown;
	};
	if (['singleImage', 'imageGrid', 'imageCarousel'].includes(block.type ?? '')) {
		const media = findMedia(block.attrs?.mediaData, key);
		if (media)
			return {
				alt:
					block.type === 'singleImage' ? (block.attrs?.altText ?? media.alt_text) : media.alt_text
			};
	}
	if (block.type === 'image' && block.attrs?.src && imageKey(block.attrs.src) === key)
		return block.attrs;
	return findContentImage(block.content, key);
}

interface Article {
	atp_uri?: string;
	atp_cid?: string;
	atp_sync_state?: string;
	atp_sync_error?: string;
	is_published?: boolean;
	is_draft?: boolean;
	published_cover?: unknown;
	content?: unknown;
}

export function offprintPageOrigin(
	page: unknown,
	imageUrl: string,
	sourceUrl: string
): ImageOrigin | undefined {
	const data = page as { url?: string; props?: { domain?: string; article?: Article } };
	if (!data?.url || data.props?.domain !== 'offprint.app') return;
	const source = new URL(sourceUrl);
	const current = new URL(data.url, source);
	if (
		current.origin + current.pathname + current.search !==
		source.origin + source.pathname + source.search
	)
		return;
	const article = data.props.article;
	if (
		!article?.atp_uri?.match(/^at:\/\/did:[^/]+\/site\.standard\.document\/[^/?#]+$/) ||
		!article.atp_cid ||
		!article.is_published ||
		article.is_draft ||
		article.atp_sync_state !== 'synced' ||
		article.atp_sync_error
	)
		return;
	const key = imageKey(imageUrl);
	const cover = findMedia(article.published_cover, key);
	const image = findContentImage(article.content, key);
	if (!cover && !image) return;
	const alt = image?.alt ?? cover?.alt_text;
	return {
		...(alt?.trim() ? { alt } : {}),
		originAtprotoRef: { uri: article.atp_uri, cid: article.atp_cid },
		originUrl: source.origin + source.pathname + source.search
	};
}

export async function resolveOffprintOrigin(
	hint: OffprintImageOriginHint
): Promise<ImageOrigin | undefined> {
	if (hint.snapshot) return hint.snapshot;
	for (const url of hint.pages) {
		try {
			if (!isOffprintUrl(url)) continue;
			const response = await fetch(url, { credentials: 'omit', signal: AbortSignal.timeout(5000) });
			if (!response.ok) continue;
			const html = await response.text();
			const json = html.match(
				/<script\b[^>]*\bdata-page=["']app["'][^>]*>([\s\S]*?)<\/script>/i
			)?.[1];
			if (!json) continue;
			const origin = offprintPageOrigin(JSON.parse(json), hint.imageUrl, response.url || url);
			if (origin) return origin;
		} catch {
			// Unavailable articles and missing provenance never prevent saving.
		}
	}
}
