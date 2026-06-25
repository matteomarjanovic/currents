import { SvelteSet } from 'svelte/reactivity';
import { apiFetch } from '$lib/api';
import type { CollectionView } from '$lib/types';

// URIs of collections deleted this session. The PDS→TAP index lags a deletion,
// so freshly-fetched lists (e.g. the profile page) still include them; surfaces
// reading collections filter against this tombstone for an immediate update.
export const deletedCollectionUris = new SvelteSet<string>();

export const collections = $state({
	items: [] as CollectionView[],
	loaded: false,
	lastUsedUri:
		(typeof localStorage !== 'undefined' ? localStorage.getItem('lastUsedCollectionUri') : null) ??
		''
});

export async function loadCollections(did: string) {
	const res = await apiFetch(
		`/xrpc/is.currents.feed.getActorCollections?actor=${encodeURIComponent(did)}&limit=100`
	);
	if (!res.ok) return;
	const data = await res.json();
	collections.items = data.collections ?? [];
	collections.loaded = true;
	if (collections.lastUsedUri === '' && collections.items.length > 0) {
		collections.lastUsedUri = collections.items[0].uri;
	}
}

export function setLastUsedCollection(uri: string) {
	collections.lastUsedUri = uri;
	localStorage.setItem('lastUsedCollectionUri', uri);
}

export function addCollection(collection: CollectionView) {
	collections.items = [collection, ...collections.items];
}

export function removeCollection(uri: string) {
	collections.items = collections.items.filter((c) => c.uri !== uri);
	deletedCollectionUris.add(uri);
}
