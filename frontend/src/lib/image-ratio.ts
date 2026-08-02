// How tall an image is allowed to render, relative to its width.
//
// Nothing clamps aspect ratio server-side — a 1:15 infographic is legitimate
// content — so the two views that show images have to decide for themselves how
// much vertical space to grant one. They pull in opposite directions:
//
//   - In a masonry grid an unbounded tile makes its column the height of the
//     whole grid, which pushes the infinite-scroll sentinel below the fold: no
//     page loads until you've scrolled the entire strip. Tiles crop instead.
//   - In the detail view `object-contain` fits by height, so past a few multiples
//     the image collapses into an unreadable sliver. Long ones fit the width and
//     scroll.
//
// Two thresholds, not one: cropping a tile is cheap (the detail view has the
// rest), so the grid clamps early; making the detail view scroll takes away a
// whole-image glance, so it waits until contain has genuinely failed.

/** Grid tiles never render taller than this many times their width. */
export const GRID_MAX_ASPECT = 2;

/** Past this, a detail view fits the width and scrolls instead of fitting the height. */
export const DETAIL_LONG_ASPECT = 3;

/** Frame/tile dimensions, clamped to `GRID_MAX_ASPECT`. Falls back to 3:4 when unknown. */
export function tileRatio(width?: number, height?: number): { width: number; height: number } {
	if (!width || !height) return { width: 3, height: 4 };
	return { width, height: Math.min(height, width * GRID_MAX_ASPECT) };
}

/** True when `tileRatio` shortened the image — the tile shows a top crop, not the whole thing. */
export function isCropped(width?: number, height?: number): boolean {
	return !!width && !!height && height > width * GRID_MAX_ASPECT;
}

/** True when a detail view should render the image full-width and let it scroll. */
export function isLongImage(width?: number, height?: number): boolean {
	return !!width && !!height && height > width * DETAIL_LONG_ASPECT;
}
