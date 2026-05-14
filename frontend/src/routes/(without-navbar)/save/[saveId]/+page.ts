import { error } from '@sveltejs/kit';
import { apiFetch } from '$lib/api';
import type { SaveView } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const uri = decodeURIComponent(params.saveId);
	const res = await apiFetch(
		`/xrpc/is.currents.feed.getSaves?uris=${encodeURIComponent(uri)}`
	);
	if (!res.ok) throw error(res.status, 'Failed to load save');
	const data = (await res.json()) as { saves: SaveView[] };
	const save = data.saves?.[0];
	if (!save) throw error(404, 'Save not found');
	return { save };
};
