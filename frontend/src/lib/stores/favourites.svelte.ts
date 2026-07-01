import { apiFetch } from '$lib/api';
import type { CollectionView } from '$lib/types';

// Collections the viewer has favourited (other users' collections). Loaded once
// per session in the organize layout; the left sidebar lists them and the page
// resolves breadcrumbs against them (they aren't in the `collections` store,
// which only holds the viewer's own collections).
export const favouriteCollections = $state({
	items: [] as CollectionView[],
	loaded: false
});

export async function loadFavouriteCollections(did: string) {
	const res = await apiFetch(
		`/xrpc/is.currents.graph.getFavouriteCollections?actor=${encodeURIComponent(did)}&limit=100`
	);
	if (!res.ok) return;
	const data = await res.json();
	favouriteCollections.items = data.collections ?? [];
	favouriteCollections.loaded = true;
}
