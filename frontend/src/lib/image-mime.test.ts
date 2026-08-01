import { describe, it, expect } from 'vitest';
import { sniffImageMime, concreteImageMime } from './image-mime';

/** Header bytes followed by padding, so length checks behave like a real file. */
function header(...bytes: number[]): Uint8Array {
	const out = new Uint8Array(32);
	out.set(bytes);
	return out;
}

const ascii = (s: string) => [...s].map((c) => c.charCodeAt(0));

const JPEG = header(0xff, 0xd8, 0xff, 0xe0);
const PNG = header(0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a);
const GIF = header(...ascii('GIF89a'));
const WEBP = header(...ascii('RIFF'), 0, 0, 0, 0, ...ascii('WEBP'));
const AVIF = header(0, 0, 0, 0x20, ...ascii('ftypavif'));
const HEIC = header(0, 0, 0, 0x18, ...ascii('ftypheic'));

describe('sniffImageMime', () => {
	const cases: [string, Uint8Array, string][] = [
		['jpeg', JPEG, 'image/jpeg'],
		['png', PNG, 'image/png'],
		['gif', GIF, 'image/gif'],
		['webp', WEBP, 'image/webp'],
		['avif', AVIF, 'image/avif'],
		['heic', HEIC, 'image/heic']
	];
	for (const [name, bytes, expected] of cases) {
		it(`detects ${name}`, () => {
			expect(sniffImageMime(bytes)).toBe(expected);
		});
	}

	it('returns null for unrecognised bytes', () => {
		expect(sniffImageMime(header(0x00, 0x01, 0x02, 0x03))).toBeNull();
	});

	it('returns null rather than reading past the end of a short buffer', () => {
		expect(sniffImageMime(new Uint8Array([0xff, 0xd8]))).toBeNull();
		expect(sniffImageMime(new Uint8Array())).toBeNull();
	});

	it('does not mistake a bare RIFF container for webp', () => {
		expect(sniffImageMime(header(...ascii('RIFF'), 0, 0, 0, 0, ...ascii('WAVE')))).toBeNull();
	});
});

describe('concreteImageMime', () => {
	it('prefers the sniffed type over a wildcard intent type', () => {
		// The bug this exists for: Android shares arrive as `image/*`, which the PDS
		// rejects against the granted `blob:image/*` scope.
		expect(concreteImageMime(PNG, 'image/*')).toBe('image/png');
	});

	it('prefers the sniffed type even over a confidently wrong declaration', () => {
		expect(concreteImageMime(PNG, 'image/jpeg')).toBe('image/png');
	});

	it('falls back to a concrete declared type when sniffing fails', () => {
		expect(concreteImageMime(header(0x00, 0x01), 'image/heic')).toBe('image/heic');
	});

	it('never returns a wildcard or an empty type', () => {
		expect(concreteImageMime(header(0x00, 0x01), 'image/*')).toBe('image/jpeg');
		expect(concreteImageMime(header(0x00, 0x01), '')).toBe('image/jpeg');
	});
});
