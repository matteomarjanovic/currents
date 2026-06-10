import { redirect } from '@sveltejs/kit';
import { PUBLIC_APPVIEW_URL } from '$env/static/public';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
	const uri = decodeURIComponent(params.uri);
	const parts = uri.split('/');
	const authority = parts[2] ?? '';
	const rkey = parts[4] ?? '';

	let handle = authority;
	const res = await fetch(
		`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.actor.getProfile?actor=${encodeURIComponent(authority)}`,
		{ credentials: 'include' }
	);
	if (res.ok) {
		const profile = await res.json();
		if (profile.handle) handle = profile.handle;
	}

	throw redirect(301, `/profile/${handle}/collection/${rkey}`);
};
