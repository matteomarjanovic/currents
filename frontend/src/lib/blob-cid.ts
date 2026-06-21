// Computes the AT Protocol blob CID for the given bytes — CIDv1, raw codec,
// sha-256 — matching what a PDS assigns on uploadBlob. Lets us check whether the
// same image already has alt text before uploading it. (Only diverges from the
// stored CID when the appview has to downscale an oversized image, > 19 MB.)

const BASE32_ALPHABET = 'abcdefghijklmnopqrstuvwxyz234567';

function base32(bytes: Uint8Array): string {
	let bits = 0;
	let value = 0;
	let out = '';
	for (const b of bytes) {
		value = (value << 8) | b;
		bits += 8;
		while (bits >= 5) {
			out += BASE32_ALPHABET[(value >>> (bits - 5)) & 31];
			bits -= 5;
		}
	}
	if (bits > 0) {
		out += BASE32_ALPHABET[(value << (5 - bits)) & 31];
	}
	return out;
}

// Web Crypto's subtle API is only exposed in secure contexts (https / localhost);
// fall back to a pure-JS sha-256 so the lookup works on any origin.
async function sha256(data: ArrayBuffer): Promise<Uint8Array> {
	if (globalThis.crypto?.subtle) {
		return new Uint8Array(await globalThis.crypto.subtle.digest('SHA-256', data));
	}
	return sha256Sync(new Uint8Array(data));
}

const rotr = (x: number, n: number) => (x >>> n) | (x << (32 - n));

const SHA256_K = new Uint32Array([
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2
]);

function sha256Sync(data: Uint8Array): Uint8Array {
	let h0 = 0x6a09e667,
		h1 = 0xbb67ae85,
		h2 = 0x3c6ef372,
		h3 = 0xa54ff53a,
		h4 = 0x510e527f,
		h5 = 0x9b05688c,
		h6 = 0x1f83d9ab,
		h7 = 0x5be0cd19;

	const l = data.length;
	const withOne = l + 1;
	const pad = (56 - (withOne % 64) + 64) % 64;
	const total = withOne + pad + 8;
	const m = new Uint8Array(total);
	m.set(data);
	m[l] = 0x80;
	const dv = new DataView(m.buffer);
	const bitLen = l * 8;
	dv.setUint32(total - 8, Math.floor(bitLen / 0x100000000));
	dv.setUint32(total - 4, bitLen >>> 0);

	const w = new Uint32Array(64);
	for (let off = 0; off < total; off += 64) {
		for (let i = 0; i < 16; i++) w[i] = dv.getUint32(off + i * 4);
		for (let i = 16; i < 64; i++) {
			const s0 = rotr(w[i - 15], 7) ^ rotr(w[i - 15], 18) ^ (w[i - 15] >>> 3);
			const s1 = rotr(w[i - 2], 17) ^ rotr(w[i - 2], 19) ^ (w[i - 2] >>> 10);
			w[i] = (w[i - 16] + s0 + w[i - 7] + s1) >>> 0;
		}
		let a = h0,
			b = h1,
			c = h2,
			d = h3,
			e = h4,
			f = h5,
			g = h6,
			h = h7;
		for (let i = 0; i < 64; i++) {
			const S1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
			const ch = (e & f) ^ (~e & g);
			const t1 = (h + S1 + ch + SHA256_K[i] + w[i]) >>> 0;
			const S0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
			const maj = (a & b) ^ (a & c) ^ (b & c);
			const t2 = (S0 + maj) >>> 0;
			h = g;
			g = f;
			f = e;
			e = (d + t1) >>> 0;
			d = c;
			c = b;
			b = a;
			a = (t1 + t2) >>> 0;
		}
		h0 = (h0 + a) >>> 0;
		h1 = (h1 + b) >>> 0;
		h2 = (h2 + c) >>> 0;
		h3 = (h3 + d) >>> 0;
		h4 = (h4 + e) >>> 0;
		h5 = (h5 + f) >>> 0;
		h6 = (h6 + g) >>> 0;
		h7 = (h7 + h) >>> 0;
	}

	const out = new Uint8Array(32);
	const odv = new DataView(out.buffer);
	[h0, h1, h2, h3, h4, h5, h6, h7].forEach((hv, i) => odv.setUint32(i * 4, hv));
	return out;
}

export async function blobCidFromBytes(bytes: ArrayBuffer): Promise<string> {
	const digest = await sha256(bytes);
	// CIDv1(0x01) + raw codec(0x55) + multihash[ sha2-256(0x12) + length(0x20) + digest ]
	const cid = new Uint8Array(4 + digest.length);
	cid.set([0x01, 0x55, 0x12, 0x20], 0);
	cid.set(digest, 4);
	return 'b' + base32(cid);
}
