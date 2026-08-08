import type { SaveView } from '$lib/types';
import { appendUnseen, countAfter, neighbourOf } from '$lib/save-sequence';

// The run of images the open detail view was opened from, so a swipe there can move
// through it. Opening a tile is a shallow route (pushState), so the grid stays mounted
// underneath and its items are already in memory — nothing here refetches what the
// viewer has already seen.
//
// The run starts as a snapshot rather than a live view of the grid's list, because the
// detail's own related rail is a grid too: tapping an image there refills that rail with
// the *new* image's related saves, and a live list would swap the run out from under the
// swipe.
//
// A grid that also hands over its `loadMore` opts into growing that snapshot: it syncs
// newly loaded items in for as long as it owns the run, and the detail asks for another
// page as the viewer nears the end. That ask is the only thing that can extend a run —
// the grid's own scroll sentinel sits under a full-screen overlay and never comes into
// view while the detail is open. Grids that hand over nothing (the related rail) stay
// fixed at whatever was loaded when the tile was tapped.
//
// The list arithmetic lives in save-sequence.ts; this module is the reactive state and
// the ownership rules around it.

/** A grid instance's identity, so a stale grid can't sync into a run it no longer owns. */
type Owner = object;

let owner: Owner | null = null;
let extend: (() => void | Promise<void>) | null = null;
let extending = false;
// Raw: the run is replaced wholesale, and deep-proxying every SaveView in it would cost
// more than it buys.
let sequence = $state.raw<SaveView[]>([]);

/** Record the grid a tile was opened from. Called just before the detail opens. */
export function setSaveSequence(
	id: Owner,
	items: SaveView[],
	loadMore?: () => void | Promise<void>
) {
	owner = id;
	extend = loadMore ?? null;
	extending = false;
	sequence = items.slice();
}

/** Fold a grid's newly loaded items into the run it owns. A no-op for any other grid. */
export function syncSaveSequence(id: Owner, items: SaveView[]) {
	if (id !== owner) return;
	sequence = appendUnseen(sequence, items);
}

/**
 * Ask the owning grid for its next page. Safe to call on every image: it collapses
 * concurrent asks, and once the grid's feed runs dry its `loadMore` is itself a no-op,
 * so this costs nothing at the true end of a run.
 */
export async function extendSaveSequence() {
	if (!extend || extending) return;
	extending = true;
	try {
		await extend();
	} finally {
		extending = false;
	}
}

/** How many images follow `uri` in the run, or -1 when it isn't part of one. */
export function savesAfter(uri: string): number {
	return countAfter(sequence, uri);
}

/**
 * The save `offset` places from `uri`, or null at either end of the run — and whenever
 * `uri` isn't in the run at all, which is how a detail view reached by deep link, with no
 * grid behind it, ends up with the gesture inert rather than walking a stale list.
 */
export function neighbourSave(uri: string, offset: number): SaveView | null {
	return neighbourOf(sequence, uri, offset);
}
