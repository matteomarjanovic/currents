import { afterEach, describe, expect, it, vi } from 'vitest';
import { uploadBlobDirect } from './blob-upload';
import { IMAGE_SIZE_ERROR, MAX_IMAGE_BYTES } from './image-size';
import { apiFetch } from '$lib/api';
import imageLexicon from '../../../lexicons/is/currents/content/image.json';

vi.mock('$lib/api', () => ({ apiFetch: vi.fn() }));
vi.mock('$lib/upload-reauth', () => ({ promptUploadReauth: vi.fn() }));
vi.mock('$lib/image-mime', () => ({
	concreteImageMime: (_bytes: Uint8Array, declared: string) => declared
}));

afterEach(() => {
	vi.clearAllMocks();
	vi.unstubAllGlobals();
});

describe('uploadBlobDirect size limit', () => {
	it('uses the image lexicon limit', () => {
		expect(MAX_IMAGE_BYTES).toBe(imageLexicon.defs.main.properties.image.maxSize);
	});

	it.each([MAX_IMAGE_BYTES + 1, 29_880_100])(
		'rejects %i bytes before requesting a token or reading/uploading the image',
		async (size) => {
			const arrayBuffer = vi.fn();
			const fetch = vi.fn();
			vi.stubGlobal('fetch', fetch);
			const file = { size, type: 'image/gif', arrayBuffer } as unknown as Blob;
			await expect(uploadBlobDirect(file)).rejects.toThrow(IMAGE_SIZE_ERROR);
			expect(apiFetch).not.toHaveBeenCalled();
			expect(fetch).not.toHaveBeenCalled();
			expect(arrayBuffer).not.toHaveBeenCalled();
		}
	);

	it('uploads an image exactly at the limit without changing its bytes', async () => {
		const bytes = new Uint8Array(MAX_IMAGE_BYTES);
		bytes.set(new TextEncoder().encode('GIF89a'));
		const file = new Blob([bytes], { type: 'image/gif' });
		vi.mocked(apiFetch).mockResolvedValueOnce(
			new Response(JSON.stringify({ token: 'test', pdsUrl: 'https://pds.test' }))
		);
		const blob = { $type: 'blob', size: MAX_IMAGE_BYTES, mimeType: 'image/gif' };
		const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ blob })));
		vi.stubGlobal('fetch', fetch);
		expect(await uploadBlobDirect(file)).toEqual(blob);
		expect(apiFetch).toHaveBeenCalledTimes(1);
		expect(fetch).toHaveBeenCalledTimes(1);
		const request = fetch.mock.calls[0][1];
		expect(request.headers['Content-Type']).toBe('image/gif');
		expect(Buffer.from(request.body).equals(Buffer.from(bytes))).toBe(true);
	});
});
