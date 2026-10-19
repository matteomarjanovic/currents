import type { ImageOrigin, LeafletImageOriginHint } from './image-origin';

export function leafletImageIdentity(raw: string) {
	const url = new URL(raw);
	if (!/^https?:$/.test(url.protocol) || url.pathname !== '/api/atproto_images') return;
	const did = url.searchParams.get('did');
	const blobCid = url.searchParams.get('cid');
	if (did?.startsWith('did:') && blobCid && !blobCid.includes('/')) return { did, blobCid };
}

export function leafletDocumentUri(raw: string) {
	return /^at:\/\/did:[^/]+\/(?:site\.standard\.document|pub\.leaflet\.document)\/[^/?#]+$/.test(
		raw
	)
		? raw
		: undefined;
}

// Leaflet advertises the source record in standard document links and AT metadata.
export function leafletHtmlDocument(html: string) {
	for (const tag of html.match(/<(?:meta|link)\b[^>]*>/gi) ?? []) {
		const attrs = new Map(
			[...tag.matchAll(/([\w:-]+)\s*=\s*["']([^"']*)["']/g)].map((m) => [m[1], m[2]])
		);
		const uri =
			attrs.get('rel') === 'site.standard.document'
				? attrs.get('href')
				: attrs.get('name') === 'at:canonical'
					? attrs.get('content')
					: undefined;
		if (uri && leafletDocumentUri(uri)) return uri;
	}
}

function hasImage(value: unknown, blobCid: string): boolean {
	if (!value || typeof value !== 'object') return false;
	if (Array.isArray(value)) return value.some((child) => hasImage(child, blobCid));
	const block = value as { $type?: string; image?: { ref?: { $link?: string } } };
	if (block.$type === 'pub.leaflet.blocks.image') return block.image?.ref?.$link === blobCid;
	return Object.values(value).some((child) => hasImage(child, blobCid));
}

async function get(url: string) {
	const response = await fetch(url, { credentials: 'omit', signal: AbortSignal.timeout(5000) });
	if (!response.ok) throw new Error(`Leaflet lookup ${response.status}`);
	return response;
}

export async function resolveLeafletOrigin(
	hint: LeafletImageOriginHint
): Promise<ImageOrigin | undefined> {
	try {
		let didUrl: string;
		if (/^did:plc:[a-z0-9]+$/.test(hint.did)) {
			didUrl = `https://plc.directory/${hint.did}`;
		} else if (hint.did.startsWith('did:web:')) {
			const [host, ...path] = hint.did.slice(8).split(':').map(decodeURIComponent);
			didUrl = `https://${host}/${path.length ? path.join('/') : '.well-known'}/did.json`;
		} else return;
		const identity = await (await get(didUrl)).json();
		if (identity.id !== hint.did) return;
		const endpoint = identity.service?.find(
			(service: { id?: string }) =>
				service.id === '#atproto_pds' || service.id === `${hint.did}#atproto_pds`
		)?.serviceEndpoint;
		if (typeof endpoint !== 'string' || new URL(endpoint).protocol !== 'https:') return;
		for (const source of hint.pages) {
			try {
				if (new URL(source).protocol !== 'https:') continue;
				let uri = hint.uri;
				let originUrl = source;
				if (!uri) {
					const response = await get(source);
					uri = leafletHtmlDocument(await response.text());
					originUrl = response.url || source;
				}
				if (!uri || !leafletDocumentUri(uri)) continue;
				const [repo, collection, rkey] = uri.slice(5).split('/');
				if (repo !== hint.did) continue;
				const params = new URLSearchParams({ repo, collection, rkey });
				const record = await (
					await get(`${endpoint.replace(/\/$/, '')}/xrpc/com.atproto.repo.getRecord?${params}`)
				).json();
				if (record.uri !== uri || !record.cid || record.value?.$type !== collection) continue;
				const value = record.value;
				const content =
					collection === 'pub.leaflet.document'
						? value.pages
						: value.content?.$type === 'pub.leaflet.content'
							? value.content
							: undefined;
				if (value.coverImage?.ref?.$link !== hint.blobCid && !hasImage(content, hint.blobCid))
					continue;
				return { originAtprotoRef: { uri, cid: record.cid }, originUrl };
			} catch {
				// A missing article or record can still leave another candidate to check.
			}
		}
	} catch {
		// Failed identity or PDS lookups never prevent saving the image.
	}
}
