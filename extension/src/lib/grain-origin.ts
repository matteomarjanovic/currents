import type { GrainImageOriginHint, ImageOrigin } from './image-origin';

export function grainImageIdentity(raw: string) {
	const url = new URL(raw);
	if (url.hostname !== 'cdn.grain.social') return;
	const match = decodeURIComponent(url.pathname).match(
		/^\/[^/]+\/feed_(?:thumbnail|fullsize)\/plain\/(did:[^/]+)\/([^/]+)$/
	);
	if (match) return { did: match[1], blobCid: match[2] };
}

interface Photo {
	uri: string;
	cid: string;
	thumb: string;
	fullsize: string;
	alt?: string;
}

export async function resolveGrainOrigin(
	hint: GrainImageOriginHint
): Promise<ImageOrigin | undefined> {
	for (const source of hint.galleries) {
		try {
			const url = new URL(source);
			const path = url.pathname.match(/^\/profile\/([^/]+)\/gallery\/([^/]+)$/);
			if (url.origin !== 'https://grain.social' || !path) continue;
			const params = new URLSearchParams({
				gallery: `at://${decodeURIComponent(path[1])}/social.grain.gallery/${path[2]}`
			});
			const response = await fetch(
				`https://grain.social/xrpc/social.grain.unspecced.getGallery?${params}`,
				{
					credentials: 'omit',
					signal: AbortSignal.timeout(5000)
				}
			);
			if (!response.ok) continue;
			const { gallery } = await response.json();
			const photos = (gallery?.items ?? []).filter((photo: Photo) => {
				if (!photo.uri?.startsWith(`at://${hint.did}/social.grain.photo/`) || !photo.cid)
					return false;
				const image = grainImageIdentity(photo.fullsize || photo.thumb);
				return image?.did === hint.did && image.blobCid === hint.blobCid;
			}) as Photo[];
			if (new Set(photos.map((photo) => photo.uri)).size !== 1) continue;
			return {
				...(photos[0].alt?.trim() ? { alt: photos[0].alt } : {}),
				originAtprotoRef: { uri: photos[0].uri, cid: photos[0].cid },
				originUrl: url.href
			};
		} catch {
			// Missing or unavailable photos never prevent saving the image.
		}
	}
}
