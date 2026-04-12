import { error } from '@sveltejs/kit';
import { PUBLIC_APPVIEW_URL } from '$env/static/public';
import type { SaveView } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
	const uri = decodeURIComponent(params.saveId);
	const url = `${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getSaves?uris=${encodeURIComponent(uri)}`;
	const res = await fetch(url, { credentials: 'include' });
	if (!res.ok) throw error(res.status, 'Failed to load save');
	const data = (await res.json()) as { saves: SaveView[] };
	const save = data.saves?.[0];
	if (!save) throw error(404, 'Save not found');
	return { save };
};
