// Concrete image type from magic bytes.
//
// The blob's mimeType rides all the way to com.atproto.repo.uploadBlob, where the PDS
// checks it against the granted `blob:image/*` scope — a wildcard (`image/*`) or generic
// type is rejected, not stored. So sniff the bytes rather than trust a declared type.
export function sniffImageMime(b: Uint8Array): string | null {
	if (b.length >= 3 && b[0] === 0xff && b[1] === 0xd8 && b[2] === 0xff) return 'image/jpeg';
	if (b.length >= 8 && b[0] === 0x89 && b[1] === 0x50 && b[2] === 0x4e && b[3] === 0x47)
		return 'image/png';
	if (b.length >= 6 && b[0] === 0x47 && b[1] === 0x49 && b[2] === 0x46) return 'image/gif';
	if (
		b.length >= 12 &&
		b[0] === 0x52 &&
		b[1] === 0x49 &&
		b[2] === 0x46 &&
		b[3] === 0x46 &&
		b[8] === 0x57 &&
		b[9] === 0x45 &&
		b[10] === 0x42 &&
		b[11] === 0x50
	)
		return 'image/webp';
	// ISO-BMFF: bytes 4..8 are "ftyp", the brand at 8..12 tells avif from heic.
	if (b.length >= 12 && b[4] === 0x66 && b[5] === 0x74 && b[6] === 0x79 && b[7] === 0x70) {
		const brand = String.fromCharCode(b[8], b[9], b[10], b[11]);
		if (brand === 'avif' || brand === 'avis') return 'image/avif';
		if (brand.startsWith('hei') || brand.startsWith('mif')) return 'image/heic';
	}
	return null;
}

/** The type to upload with: sniffed first, the declared type only when it's concrete. */
export function concreteImageMime(bytes: Uint8Array, declared: string): string {
	const sniffed = sniffImageMime(bytes);
	if (sniffed) return sniffed;
	if (declared && !declared.includes('*')) return declared;
	return 'image/jpeg';
}
