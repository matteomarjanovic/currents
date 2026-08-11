// Finds the images worth offering in the multi-image picker.
//
// The filter is the feature: unfiltered, a typical page yields a hundred icons,
// avatars, sprites and tracking pixels, and the user abandons the panel rather
// than deselect sixty tiles. Everything here errs towards dropping a borderline
// image — the right-click-an-image path still saves anything by hand.

export interface ImageCandidate {
	url: string;
	alt: string;
	// Natural size where known, else the rendered box (lazy images that haven't
	// loaded yet). Shown on the tile so low-res images are easy to spot.
	width: number;
	height: number;
}

// Smallest side worth offering: kills icons, avatars, spacers and 1×1 pixels.
const MIN_DIM = 200;
// Anything longer than this in either direction is a banner, rule or strip.
const MAX_ASPECT = 4;
// Enough for any real gallery, and keeps both the grid and the upload run bounded.
const MAX_IMAGES = 100;

function absolute(url: string, base: string): string {
	try {
		return new URL(url, base).href;
	} catch {
		return url;
	}
}

// Largest entry in the srcset (sites list 236x → originals), falling back to
// the rendered src. srcset URLs may be relative, so the result is resolved.
export function bestImageUrl(img: HTMLImageElement): string {
	let best = img.currentSrc || img.src;
	let bestDesc = 0;
	for (const part of (img.srcset ?? '').split(',')) {
		const [url, desc] = part.trim().split(/\s+/);
		const d = parseFloat(desc) || 0;
		if (url && d > bestDesc) {
			bestDesc = d;
			best = url;
		}
	}
	return absolute(best, img.baseURI);
}

export function collectPageImages(doc: Document = document): ImageCandidate[] {
	const out: ImageCandidate[] = [];
	const seen = new Set<string>();

	for (const img of Array.from(doc.images)) {
		if (out.length >= MAX_IMAGES) break;

		// No box means display:none or detached; hidden means it's there but unseen.
		const rect = img.getBoundingClientRect();
		if (rect.width === 0 || rect.height === 0) continue;
		if (getComputedStyle(img).visibility === 'hidden') continue;
		// Finished loading with no intrinsic size = broken.
		if (img.complete && img.naturalWidth === 0) continue;

		const width = img.naturalWidth || Math.round(rect.width);
		const height = img.naturalHeight || Math.round(rect.height);
		if (width < MIN_DIM || height < MIN_DIM) continue;
		const aspect = width / height;
		if (aspect > MAX_ASPECT || aspect < 1 / MAX_ASPECT) continue;

		const url = bestImageUrl(img);
		// blob: URLs are scoped to the page's own origin — the background worker
		// that performs the upload cannot fetch them.
		if (!/^(https?:|data:)/.test(url)) continue;
		if (seen.has(url)) continue;
		seen.add(url);

		out.push({ url, alt: img.alt.trim().slice(0, 2000), width, height });
	}

	return out;
}
