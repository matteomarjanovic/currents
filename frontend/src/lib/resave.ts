import { apiFetch } from '$lib/api';
import { concreteImageMime } from '$lib/image-mime';
import { promptUploadReauth } from '$lib/upload-reauth';

function base64ToBytes(b64: string): Uint8Array<ArrayBuffer> {
	const bin = atob(b64);
	const out = new Uint8Array(bin.length);
	for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
	return out;
}

// Resave an existing save into a collection. Normally a single POST /resave the
// appview handles server-side. When the appview's shared per-IP blob bucket is
// exhausted it answers 429 with the image bytes it already fetched plus a
// service-auth token; we then upload the blob from the user's own IP (their own
// bucket) and retry with the resulting blob ref. Returns the final Response so
// callers keep their existing status handling — a synthetic Response carries the
// status when the client-side upload itself fails (e.g. the user's own bucket).
export async function resaveWithFallback(
	saveUri: string,
	collectionUri: string
): Promise<Response> {
	const first = await apiFetch('/api/resave', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ saveUri, collectionUri })
	});
	if (first.status !== 429) return first;

	let payload: {
		rateLimited?: boolean;
		needsReauth?: boolean;
		token?: string;
		pdsUrl?: string;
		image?: string;
		contentType?: string;
	};
	try {
		payload = await first.clone().json();
	} catch {
		return first; // plain-text 429 from an older server — surface it as-is
	}
	// Rate-limited and the session lacks the rpc: scope, so the client-side rescue
	// can't run — reconnecting would unlock it, so nudge and surface the 429.
	if (payload.needsReauth) {
		promptUploadReauth();
		return first;
	}
	if (!payload.rateLimited || !payload.token || !payload.pdsUrl || !payload.image) return first;

	try {
		const bytes = base64ToBytes(payload.image);
		const up = await fetch(
			`${payload.pdsUrl.replace(/\/$/, '')}/xrpc/com.atproto.repo.uploadBlob`,
			{
				method: 'POST',
				headers: {
					Authorization: `Bearer ${payload.token}`,
					'Content-Type': concreteImageMime(bytes, payload.contentType ?? '')
				},
				body: bytes
			}
		);
		if (!up.ok) return new Response(null, { status: up.status });
		const { blob } = await up.json();
		return await apiFetch('/api/resave', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ saveUri, collectionUri, blob })
		});
	} catch {
		return first; // fallback failed unexpectedly — surface the original 429
	}
}
