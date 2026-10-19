import { BLUESKY_APIS } from '../../lib/bluesky-origin';
import type { ImageOriginHint } from '../../lib/image-origin';

function imageIdentity(img: HTMLImageElement) {
	const url = new URL(img.currentSrc || img.src, location.href);
	let did: string | undefined, blobCid: string | undefined;
	if (url.pathname === '/xrpc/com.atproto.sync.getBlob') {
		did = url.searchParams.get('did') ?? undefined;
		blobCid = url.searchParams.get('cid') ?? undefined;
	} else if (url.hostname === 'cdn.bsky.app') {
		const match = decodeURIComponent(url.pathname).match(
			/^\/img\/feed_(?:thumbnail|fullsize)\/plain\/(did:[^/]+)\/([^/@]+)(?:@[^/]+)?$/
		);
		did = match?.[1];
		blobCid = match?.[2];
	}
	if (!did?.startsWith('did:') || !blobCid) return;
	return { did, blobCid };
}

export function blueskyImageOrigin(img: HTMLImageElement): ImageOriginHint | undefined {
	if (!Object.hasOwn(BLUESKY_APIS, location.hostname)) return;
	if (img.closest('[data-testid="userAvatarImage"]')) return;
	const identity = imageIdentity(img);
	if (!identity) return;
	const { did, blobCid } = identity;

	if (location.hostname === 'tokimeki.blue') {
		let uri = img.closest('[data-aturi]')?.getAttribute('data-aturi');
		if (!uri) {
			// The image viewer is outside the timeline DOM. Match its blob to a
			// mounted post image, but leave ambiguous/reused images unattributed.
			const owners = new Set<string>();
			for (const other of document.images) {
				const owner = other.closest('[data-aturi]')?.getAttribute('data-aturi');
				if (!owner) continue;
				const otherIdentity = imageIdentity(other);
				if (otherIdentity?.did === did && otherIdentity.blobCid === blobCid) owners.add(owner);
			}
			if (owners.size === 1) uri = [...owners][0];
		}
		const record = uri?.match(/^at:\/\/(did:[^/]+)\/app\.bsky\.feed\.post\/([^/?#]+)$/);
		if (record) {
			return {
				app: 'bluesky',
				client: location.hostname,
				did,
				blobCid,
				posts: [`https://${location.hostname}/profile/${record[1]}/post/${record[2]}`]
			};
		}
		return;
	}

	const posts = new Set<string>();
	const add = (href: string) => {
		const link = new URL(href, location.href);
		const path = link.pathname.match(/^\/profile\/[^/]+\/post\/[^/]+(?=\/|$)/)?.[0];
		if (Object.hasOwn(BLUESKY_APIS, link.hostname) && path) {
			posts.add(link.origin + path);
		}
	};
	let anisotaCard =
		location.hostname === 'anisota.net' ? img.closest('.embedded-quote, .post-card') : null;
	if (anisotaCard?.matches('.embedded-quote')) {
		anisotaCard = anisotaCard.closest('.embedded-quote-link') ?? anisotaCard;
	}
	const container =
		anisotaCard ?? img.closest('[data-testid^="postThreadItem"], [data-testid^="feedItem"]');
	// Use the nearest post's links, rather than every post in the feed. The
	// resolver verifies blob membership, including images inside quoted posts.
	for (
		let node: HTMLElement | null = img;
		node && node !== document.body;
		node = node.parentElement
	) {
		if (node instanceof HTMLAnchorElement) add(node.href);
		for (const link of node.querySelectorAll<HTMLAnchorElement>('a[href*="/post/"]')) {
			if (anisotaCard) {
				const scope =
					link.closest('.embedded-quote-link') ?? link.closest('.embedded-quote, .post-card');
				if (scope !== anisotaCard) continue;
			}
			add(link.href);
		}
		// Anisota places quotes before the enclosing post's footer permalink.
		if (node === container || (!anisotaCard && posts.size)) break;
	}
	// A lightbox can live outside the post DOM; its route identifies the post.
	add(location.href);
	if (!posts.size) return;
	return { app: 'bluesky', client: location.hostname, did, blobCid, posts: [...posts].slice(0, 8) };
}
