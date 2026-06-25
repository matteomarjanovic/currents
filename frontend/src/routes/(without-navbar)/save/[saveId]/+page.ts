import { redirect } from '@sveltejs/kit';
import { apiFetch } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const uri = decodeURIComponent(params.saveId);
	const parts = uri.split('/');
	const authority = parts[2] ?? '';
	const rkey = parts[4] ?? '';

	let handle = authority;
	const res = await apiFetch(
		`/xrpc/is.currents.actor.getProfile?actor=${encodeURIComponent(authority)}`
	);
	if (res.ok) {
		const profile = await res.json();
		if (profile.handle) handle = profile.handle;
	}

	throw redirect(301, `/profile/${handle}/save/${rkey}`);
};
