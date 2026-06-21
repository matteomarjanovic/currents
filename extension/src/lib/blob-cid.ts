// Computes the AT Protocol blob CID for the given bytes — CIDv1, raw codec,
// sha-256 — matching what a PDS assigns on uploadBlob. Lets us check whether the
// same image already has alt text before saving it.

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

export async function blobCidFromBytes(bytes: ArrayBuffer): Promise<string> {
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes));
  // CIDv1(0x01) + raw codec(0x55) + multihash[ sha2-256(0x12) + length(0x20) + digest ]
  const cid = new Uint8Array(4 + digest.length);
  cid.set([0x01, 0x55, 0x12, 0x20], 0);
  cid.set(digest, 4);
  return 'b' + base32(cid);
}
