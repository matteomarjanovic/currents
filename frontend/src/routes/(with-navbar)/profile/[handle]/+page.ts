import { apiFetch } from '$lib/api';
import type { ActorProfileView, CollectionView } from '$lib/types';
import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
	const handle = params.handle;

	const profileRes = await apiFetch(
		`/xrpc/is.currents.actor.getProfile?actor=${encodeURIComponent(handle)}`,
		{},
		fetch
	);
	if (!profileRes.ok) throw error(profileRes.status, 'Profile not found');

	const profile = (await profileRes.json()) as ActorProfileView;
	const collections: CollectionView[] = [];
	let cursor = '';
	do {
		const query = new URLSearchParams({ actor: handle, parent: 'root', limit: '100' });
		if (cursor) query.set('cursor', cursor);
		const res = await apiFetch(`/xrpc/is.currents.feed.getActorCollections?${query}`, {}, fetch);
		if (!res.ok) break;
		const data = await res.json();
		collections.push(...(data.collections ?? []));
		cursor = data.cursor ?? '';
	} while (cursor);

	return { profile, collections };
};
