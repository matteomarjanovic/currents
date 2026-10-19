import type { ImageOrigin, PcktImageOriginHint } from './image-origin';

export function isPcktUrl(raw: string) {
	const url = new URL(raw);
	return (
		url.protocol === 'https:' &&
		(url.hostname === 'pckt.blog' || url.hostname.endsWith('.pckt.blog'))
	);
}

function imageKey(raw: string) {
	const url = new URL(raw);
	// Pckt serves originals and converted versions under one immutable media ID.
	if (url.hostname === 'pckt-blog-media.s3.us-east-2.amazonaws.com') {
		const asset = url.pathname.match(/^\/(images|cover_image)\/([^/]+)\//)?.[0];
		if (asset) return url.origin + asset;
	}
	return url.href;
}

function contentHasImage(node: unknown, key: string): boolean {
	if (!node || typeof node !== 'object') return false;
	if (Array.isArray(node)) return node.some((child) => contentHasImage(child, key));
	const block = node as {
		type?: string;
		attrs?: { src?: string; images?: { src?: string; url?: string }[] };
		content?: unknown;
	};
	if ((block.type === 'imageUpload' || block.type === 'image') && block.attrs?.src) {
		if (imageKey(block.attrs.src) === key) return true;
	}
	if (
		block.attrs?.images?.some((image) => {
			const src = image.src || image.url;
			return src && imageKey(src) === key;
		})
	)
		return true;
	return contentHasImage(block.content, key);
}

interface Post {
	atproto_uri?: string;
	atproto_cid?: string;
	is_published?: boolean;
	is_syncing_to_atproto?: boolean;
	atproto_sync_error?: string;
	cover_image_url?: string;
	cover_image_original_url?: string;
	content?: string | object;
}

export function pcktPageOrigin(
	page: unknown,
	imageUrl: string,
	sourceUrl: string
): ImageOrigin | undefined {
	const data = page as { url?: string; props?: { app_url?: string; post?: Post } };
	if (!data?.url || !data.props?.app_url || !isPcktUrl(data.props.app_url)) return;
	const source = new URL(sourceUrl);
	const current = new URL(data.url, source);
	if (
		current.origin + current.pathname + current.search !==
		source.origin + source.pathname + source.search
	)
		return;
	const post = data.props.post;
	if (
		!post?.atproto_uri?.match(
			/^at:\/\/did:[^/]+\/(?:site\.standard\.document|blog\.pckt\.document)\/[^/?#]+$/
		) ||
		!post.atproto_cid ||
		!post.is_published ||
		post.is_syncing_to_atproto ||
		post.atproto_sync_error
	)
		return;
	const key = imageKey(imageUrl);
	const cover = [post.cover_image_url, post.cover_image_original_url].some(
		(src) => src && imageKey(src) === key
	);
	if (!cover) {
		const content = typeof post.content === 'string' ? JSON.parse(post.content) : post.content;
		if (!contentHasImage(content, key)) return;
	}
	return {
		originAtprotoRef: { uri: post.atproto_uri, cid: post.atproto_cid },
		originUrl: source.origin + source.pathname + source.search
	};
}

export async function resolvePcktOrigin(
	hint: PcktImageOriginHint
): Promise<ImageOrigin | undefined> {
	if (hint.snapshot) return hint.snapshot;
	for (const url of hint.pages) {
		try {
			if (!isPcktUrl(url)) continue;
			const response = await fetch(url, { credentials: 'omit', signal: AbortSignal.timeout(5000) });
			if (!response.ok) continue;
			const html = await response.text();
			const json = html.match(
				/<script\b[^>]*\bdata-page=["']app["'][^>]*>([\s\S]*?)<\/script>/i
			)?.[1];
			if (!json) continue;
			const origin = pcktPageOrigin(JSON.parse(json), hint.imageUrl, response.url || url);
			if (origin) return origin;
		} catch {
			// Unpublished posts, missing metadata and failed lookups remain optional.
		}
	}
}
