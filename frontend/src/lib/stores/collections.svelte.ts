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
	// The selector needs every collection (roots and sections), so page through
	// the full set instead of capping at one 100-item page — accounts with more
	// than 100 collections would otherwise be silently truncated.
	const all: CollectionView[] = [];
	let cursor = '';
	do {
		const params = new URLSearchParams({ actor: did, limit: '100' });
		if (cursor) params.set('cursor', cursor);
		const res = await apiFetch(`/xrpc/is.currents.feed.getActorCollections?${params}`);
		if (!res.ok) return;
		const data = await res.json();
		all.push(...(data.collections ?? []));
		cursor = data.cursor ?? '';
	} while (cursor);
	collections.items = all;
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

// Apply an edit locally. The PDS→TAP index lags a write, so refetching right
// after a rename or a move would hand back the pre-edit row.
export function updateCollection(uri: string, patch: Partial<CollectionView>) {
	collections.items = collections.items.map((c) => (c.uri === uri ? { ...c, ...patch } : c));
}

export async function setCollectionPinned(uri: string, pinned: boolean): Promise<boolean> {
	const collection = collections.items.find((c) => c.uri === uri);
	if (!collection) return false;
	const previous = !!collection.viewer?.pinned;
	updateCollection(uri, { viewer: { ...collection.viewer, pinned } });
	try {
		const res = await apiFetch('/api/collections/pinned', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ collectionUri: uri, pinned })
		});
		if (!res.ok) throw new Error('pin request failed');
		return true;
	} catch {
		updateCollection(uri, { viewer: { ...collection.viewer, pinned: previous } });
		return false;
	}
}

export function removeCollection(uri: string) {
	collections.items = collections.items.filter((c) => c.uri !== uri);
	deletedCollectionUris.add(uri);
}
