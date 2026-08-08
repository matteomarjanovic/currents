import { apiFetch } from '$lib/api';
import { concreteImageMime } from '$lib/image-mime';

// A failure in the direct-to-PDS upload flow. `status` is the HTTP status of
// whichever step failed; `phase` says which one, so callers can preserve the
// existing UX (401 → re-auth, 429 → the PDS is rate-limiting this user).
export class DirectUploadError extends Error {
	constructor(
		readonly status: number,
		readonly phase: 'token' | 'upload'
	) {
		super(`direct blob upload failed at ${phase} (HTTP ${status})`);
		this.name = 'DirectUploadError';
	}
}

// Upload an image blob straight to the user's PDS using a short-lived
// service-auth token minted by the appview, so com.atproto.repo.uploadBlob runs
// from the user's own IP — their own per-IP rate-limit bucket — instead of the
// shared appview IP. Returns the PDS's blob ref, to be sent to POST /save as the
// `blob` field in place of the raw `image` file.
export async function uploadBlobDirect(file: Blob): Promise<unknown> {
	const tokenRes = await apiFetch('/api/blob/upload-token', { method: 'POST' });
	if (!tokenRes.ok) throw new DirectUploadError(tokenRes.status, 'token');
	const { token, pdsUrl } = await tokenRes.json();

	const bytes = new Uint8Array(await file.arrayBuffer());
	const res = await fetch(`${pdsUrl.replace(/\/$/, '')}/xrpc/com.atproto.repo.uploadBlob`, {
		method: 'POST',
		headers: {
			Authorization: `Bearer ${token}`,
			'Content-Type': concreteImageMime(bytes, file.type)
		},
		body: bytes
	});
	if (!res.ok) throw new DirectUploadError(res.status, 'upload');
	return (await res.json()).blob;
}
