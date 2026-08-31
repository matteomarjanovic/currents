import { SvelteSet } from 'svelte/reactivity';
import { toast } from 'svelte-sonner';
import { apiFetch } from '$lib/api';
import { getImageContent, type SaveView } from '$lib/types';
import { auth } from '$lib/stores/auth.svelte';
import { promptLogin } from '$lib/stores/login-prompt.svelte';

const hiddenBlobCids = new SvelteSet<string>();

export function isHiddenFeedImage(save: SaveView): boolean {
	const cid = getImageContent(save)?.blobCid;
	return !!cid && hiddenBlobCids.has(cid);
}

export async function hideFeedImage(save: SaveView): Promise<boolean> {
	if (!auth.user) {
		promptLogin();
		return false;
	}
	try {
		const res = await apiFetch('/api/feed/hidden', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ uri: save.uri })
		});
		if (!res.ok) {
			if (res.status === 401) {
				auth.user = null;
				promptLogin();
				return false;
			}
			toast.error(`Couldn't hide image (${res.status}).`);
			return false;
		}
		const cid = getImageContent(save)?.blobCid;
		if (cid) hiddenBlobCids.add(cid);
		toast.success('Image hidden from your feed');
		return true;
	} catch {
		toast.error("Couldn't hide image. Please try again.");
		return false;
	}
}
