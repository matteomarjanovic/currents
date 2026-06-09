import { error } from '@sveltejs/kit';
import { PUBLIC_APPVIEW_URL } from '$env/static/public';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
	const res = await fetch(
		`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.actor.getProfile?actor=${encodeURIComponent(params.handle)}`,
		{ credentials: 'include' }
	);
	if (!res.ok) throw error(res.status, 'Profile not found');
	const profile = await res.json();
	return {
		collectionUri: `at://${profile.did}/is.currents.feed.collection/${params.rkey}`
	};
};
