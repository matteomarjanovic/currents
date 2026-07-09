import { afterEach, describe, expect, it, vi } from 'vitest';
import { blobCidFromBytes } from './blob-cid';

// Expected CIDs computed independently (Python: hashlib sha-256 + CIDv1 raw
// multiformats layout + lowercase unpadded base32). Viewer save state matches
// images by this CID, so any drift here silently breaks resave detection.
const VECTORS: { name: string; bytes: Uint8Array; cid: string }[] = [
	{
		name: 'empty',
		bytes: new Uint8Array(0),
		cid: 'bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku'
	},
	{
		name: 'hello world',
		bytes: new TextEncoder().encode('hello world'),
		cid: 'bafkreifzjut3te2nhyekklss27nh3k72ysco7y32koao5eei66wof36n5e'
	},
	{
		// 100 bytes: exercises the two-block padding path (length > 55)
		name: 'bytes 0..99',
		bytes: new Uint8Array(Array.from({ length: 100 }, (_, i) => i)),
		cid: 'bafkreif44cx7dhhvvjvhi2ndbvq5atsdo3slx5rycbjo5ht7gojfzfknki'
	},
	{
		// 129 bytes: crosses two full sha-256 blocks plus a padding block
		name: '129 × 0xab',
		bytes: new Uint8Array(129).fill(0xab),
		cid: 'bafkreida4kb7lp4qp3arfqo56i5ce66d3n3d6tob7p3dmzzbyr5l3qfvgy'
	}
];

function toBuffer(bytes: Uint8Array): ArrayBuffer {
	// Copy into an exactly-sized buffer — .buffer of a view may be larger.
	return bytes.slice().buffer;
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('blobCidFromBytes', () => {
	describe('via WebCrypto', () => {
		for (const v of VECTORS) {
			it(v.name, async () => {
				await expect(blobCidFromBytes(toBuffer(v.bytes))).resolves.toBe(v.cid);
			});
		}
	});

	describe('via the pure-JS sha-256 fallback (insecure contexts)', () => {
		for (const v of VECTORS) {
			it(v.name, async () => {
				vi.stubGlobal('crypto', undefined);
				await expect(blobCidFromBytes(toBuffer(v.bytes))).resolves.toBe(v.cid);
			});
		}
	});
});
