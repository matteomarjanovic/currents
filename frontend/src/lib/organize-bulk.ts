import { getImageContent, type SaveView } from './types';

// What the action bar needs to act on a selection. The canvas owns the feed these
// are derived from, but the bar renders as a sibling of the canvas's rounded panel,
// so the canvas hands this up (live, via getters) for the page to pass along.
export type BulkApi = {
	readonly saves: SaveView[];
	readonly selectableCount: number;
	readonly canMove: boolean;
	onSelectAll: () => void;
	onClear: () => void;
	onExit: () => void;
	onCopy: (dest: string) => void;
	onMove: (dest: string) => void;
};

// Resolve a selection Set of save URIs against a list of loaded saves, preserving
// the list's order. Selections can outlive a feed page (a URI stays selected while
// its tile scrolls), so anything no longer present is simply dropped.
export function selectedSaves(items: SaveView[], selected: Set<string>): SaveView[] {
	return items.filter((i) => selected.has(i.uri));
}

// URIs eligible for multi-select: only image saves (unsupported content can't be
// copied/moved/downloaded/labeled). Used by "select all loaded".
export function selectableUris(items: SaveView[]): string[] {
	return items.filter((i) => !!getImageContent(i)).map((i) => i.uri);
}

// The distinct blob CIDs among the given saves. Attribution is keyed by blob CID
// (one PUT /save/attribution fans out over every rkey of that blob), so a bulk
// attribution only needs one call per distinct blob.
export function distinctBlobCids(saves: SaveView[]): string[] {
	const seen = new Set<string>();
	for (const s of saves) {
		const cid = getImageContent(s)?.blobCid;
		if (cid) seen.add(cid);
	}
	return [...seen];
}

// The rkeys (record keys) for the given saves — the shape PUT /save/labels/bulk
// expects. Non-image saves are excluded (nothing to label).
export function saveRkeys(saves: SaveView[]): string[] {
	return saves
		.filter((s) => !!getImageContent(s))
		.map((s) => s.uri.split('/').pop())
		.filter((r): r is string => !!r);
}
