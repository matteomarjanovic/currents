#!/usr/bin/env node
// Generates AT Protocol TIDs (record keys) in advance, so a document's rkey can be committed to
// the .svx frontmatter before the corresponding site.standard.document record is published.
//
// Format per the atproto spec: a 64-bit value — top bit 0, next 53 bits a microsecond-precision
// UNIX timestamp, final 10 bits a random clock id (avoids collisions between generators running
// at the same microsecond) — encoded as 13 characters of base32-sortable (lexicographic order
// matches numeric order), alphabet `234567abcdefghijklmnopqrstuvwxyz`.

const S32_CHARS = '234567abcdefghijklmnopqrstuvwxyz';

function s32encode(value) {
	let s = '';
	let n = value;
	for (let i = 0; i < 13; i++) {
		s = S32_CHARS[Number(n & 31n)] + s;
		n >>= 5n;
	}
	return s;
}

function genTid() {
	const micros = BigInt(Date.now()) * 1000n;
	const clockId = BigInt(Math.floor(Math.random() * 1024)); // 10 bits
	return s32encode((micros << 10n) | clockId);
}

const count = Number(process.argv[2] ?? 1);
for (let i = 0; i < count; i++) console.log(genTid());
