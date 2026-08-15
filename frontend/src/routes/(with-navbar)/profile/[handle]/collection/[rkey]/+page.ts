import { error } from '@sveltejs/kit';
import { apiFetch } from '$lib/api';
import type { CollectionView, SaveView } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
	const res = await apiFetch(
		`/xrpc/is.currents.actor.getProfile?actor=${encodeURIComponent(params.handle)}`,
		{},
		fetch
	);
	if (!res.ok) throw error(res.status, 'Profile not found');
	const profile = await res.json();
	const collectionUri = `at://${profile.did}/is.currents.feed.collection/${params.rkey}`;

	const pageParams = new URLSearchParams({ collection: collectionUri, limit: '50' });
	const savesRes = await apiFetch(
		`/xrpc/is.currents.feed.getCollectionSaves?${pageParams}`,
		{},
		fetch
	);
	if (savesRes.status === 404) {
		throw error(404, 'Collection not found');
	}
	if (!savesRes.ok) {
		return {
			collectionUri,
			collection: null,
			saves: [],
			cursor: undefined,
			children: [],
			loadError: true
		};
	}

	const page = (await savesRes.json()) as {
		collection?: CollectionView;
		saves?: SaveView[];
		cursor?: string;
	};
	const children: CollectionView[] = [];
	let cursor = '';
	do {
		const query = new URLSearchParams({
			actor: profile.did,
			parent: collectionUri,
			limit: '100'
		});
		if (cursor) query.set('cursor', cursor);
		const childRes = await apiFetch(
			`/xrpc/is.currents.feed.getActorCollections?${query}`,
			{},
			fetch
		);
		if (!childRes.ok) break;
		const data = await childRes.json();
		children.push(...(data.collections ?? []));
		cursor = data.cursor ?? '';
	} while (cursor);

	return {
		collectionUri,
		collection: page.collection ?? null,
		saves: page.saves ?? [],
		cursor: page.cursor,
		children
	};
};
