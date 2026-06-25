// Lightweight event bus so already-rendered grids (e.g. the collection page, profile Unsorted
// tab) can drop an item the moment its save record is deleted elsewhere — typically when the
// image is unsaved from the <SaveDetail> overlay, which lives in the layout and can't reach the
// page's list directly. Avoids a full refetch / stale list. Matched by the deleted record's
// `saveUri`, scoped to the `collectionUri` it left ('' = the profile/unsorted bucket).
export type SaveRemovedEvent = { saveUri: string; collectionUri: string };

const removedListeners = new Set<(e: SaveRemovedEvent) => void>();

export function onSaveRemoved(cb: (e: SaveRemovedEvent) => void): () => void {
	removedListeners.add(cb);
	return () => {
		removedListeners.delete(cb);
	};
}

export function emitSaveRemoved(event: SaveRemovedEvent): void {
	for (const cb of removedListeners) cb(event);
}
