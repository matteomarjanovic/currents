import { error } from '@sveltejs/kit';
import { apiFetch } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const res = await apiFetch(
		`/xrpc/is.currents.actor.getProfile?actor=${encodeURIComponent(params.handle)}`
	);
	if (!res.ok) throw error(res.status, 'Profile not found');
	const profile = await res.json();
	return {
		collectionUri: `at://${profile.did}/is.currents.feed.collection/${params.rkey}`
	};
};
