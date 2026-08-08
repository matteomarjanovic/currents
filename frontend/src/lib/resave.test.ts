import { afterEach, describe, expect, it, vi } from 'vitest';
import { resaveWithFallback } from './resave';
import { apiFetch } from '$lib/api';

vi.mock('$lib/api', () => ({ apiFetch: vi.fn() }));
vi.mock('$lib/image-mime', () => ({
	concreteImageMime: (_bytes: Uint8Array, declared: string) => declared || 'image/jpeg'
}));

const mockApiFetch = vi.mocked(apiFetch);
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

function json(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

afterEach(() => {
	vi.clearAllMocks();
});

describe('resaveWithFallback', () => {
	it('returns the server response directly when not rate-limited', async () => {
		mockApiFetch.mockResolvedValueOnce(json({ uri: 'at://x' }));
		const res = await resaveWithFallback('at://save', 'at://coll');
		expect(res.status).toBe(200);
		expect(mockApiFetch).toHaveBeenCalledTimes(1);
		expect(mockFetch).not.toHaveBeenCalled(); // no client-side upload
	});

	it('surfaces a non-429 error without falling back', async () => {
		mockApiFetch.mockResolvedValueOnce(json({}, 401));
		const res = await resaveWithFallback('at://save', 'at://coll');
		expect(res.status).toBe(401);
		expect(mockFetch).not.toHaveBeenCalled();
	});

	it('on a structured 429, uploads to the PDS from the client and retries with the blob ref', async () => {
		mockApiFetch
			.mockResolvedValueOnce(
				json(
					{
						rateLimited: true,
						token: 'svc-jwt',
						pdsUrl: 'https://pds.example/',
						image: btoa('rawbytes'),
						contentType: 'image/jpeg'
					},
					429
				)
			)
			.mockResolvedValueOnce(json({ uri: 'at://new' }));
		mockFetch.mockResolvedValueOnce(json({ blob: { $type: 'blob', ref: { $link: 'cid' } } }));

		const res = await resaveWithFallback('at://save', 'at://coll');
		expect(res.status).toBe(200);
		expect(await res.json()).toEqual({ uri: 'at://new' });

		// Uploaded straight to the PDS with the service-auth token (trailing slash collapsed).
		const [url, init] = mockFetch.mock.calls[0];
		expect(url).toBe('https://pds.example/xrpc/com.atproto.repo.uploadBlob');
		expect((init.headers as Record<string, string>).Authorization).toBe('Bearer svc-jwt');

		// The retry carries the blob ref so the appview skips its own upload.
		const retryBody = JSON.parse(mockApiFetch.mock.calls[1][1]!.body as string);
		expect(retryBody).toMatchObject({
			saveUri: 'at://save',
			collectionUri: 'at://coll',
			blob: { $type: 'blob' }
		});
	});

	it("reports the PDS status when the client's own upload is rate-limited too", async () => {
		mockApiFetch.mockResolvedValueOnce(
			json({ rateLimited: true, token: 't', pdsUrl: 'https://pds.example', image: btoa('x') }, 429)
		);
		mockFetch.mockResolvedValueOnce(new Response(null, { status: 429 }));

		const res = await resaveWithFallback('at://save', 'at://coll');
		expect(res.status).toBe(429);
		expect(mockApiFetch).toHaveBeenCalledTimes(1); // no retry
	});

	it('surfaces a plain-text 429 from an older server unchanged', async () => {
		mockApiFetch.mockResolvedValueOnce(new Response('rate limited', { status: 429 }));
		const res = await resaveWithFallback('at://save', 'at://coll');
		expect(res.status).toBe(429);
		expect(mockFetch).not.toHaveBeenCalled();
	});
});
