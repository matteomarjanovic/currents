import { apiFetch } from '$lib/api';
import type { ActorProfileView, CollectionView } from '$lib/types';
import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

const PROFILE_COLLECTION_PAGE = 16;

export const load: PageLoad = async ({ params, fetch }) => {
	const handle = params.handle;

	const profileRes = await apiFetch(
		`/xrpc/is.currents.actor.getProfile?actor=${encodeURIComponent(handle)}`,
		{},
		fetch
	);
	if (!profileRes.ok) throw error(profileRes.status, 'Profile not found');

	const profile = (await profileRes.json()) as ActorProfileView;
	const query = new URLSearchParams({
		actor: handle,
		parent: 'root',
		limit: String(PROFILE_COLLECTION_PAGE)
	});
	const collectionsRes = await apiFetch(
		`/xrpc/is.currents.feed.getActorCollections?${query}`,
		{},
		fetch
	);
	const collectionsPage = collectionsRes.ok
		? ((await collectionsRes.json()) as { collections?: CollectionView[]; cursor?: string })
		: {};

	return {
		profile,
		collections: collectionsPage.collections ?? [],
		collectionsCursor: collectionsPage.cursor
	};
};
