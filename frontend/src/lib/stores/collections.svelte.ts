import { PUBLIC_APPVIEW_URL } from '$env/static/public';
import type { CollectionView } from '$lib/types';

export const collections = $state({
	items: [] as CollectionView[],
	loaded: false,
	lastUsedUri:
		(typeof localStorage !== 'undefined' ? localStorage.getItem('lastUsedCollectionUri') : null) ??
		''
});

export async function loadCollections(did: string) {
	const res = await fetch(
		`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getActorCollections?actor=${encodeURIComponent(did)}&limit=100`,
		{ credentials: 'include' }
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
