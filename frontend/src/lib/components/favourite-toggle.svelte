<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { Toggle } from '$lib/components/ui/toggle';
	import Star from '@lucide/svelte/icons/star';
	import FollowScopeDialog from './follow-scope-dialog.svelte';
	import { favouriteCollection, unfavouriteCollection } from '$lib/favourite';
	import type { CollectionView } from '$lib/types';

	// Star toggle for favouriting another user's collection. Owns the optimistic
	// state + scope-missing re-auth; `onChange` lets the parent update a count it
	// displays elsewhere. The text label appears from `labelFrom` up (star-only
	// below): `sm` by default, or `never` for a permanent icon-only button in tight
	// rows like the save-detail collection list. Callers must only render this for
	// collections the viewer can favourite (logged in, not their own — the backend
	// rejects self-favourites).
	interface Props {
		collection: CollectionView;
		onChange?: (favourited: boolean) => void;
		labelFrom?: 'sm' | 'never';
	}
	let { collection, onChange, labelFrom = 'sm' }: Props = $props();
	// Literal classes (not interpolated) so Tailwind keeps them in the build.
	const labelClass = $derived(labelFrom === 'never' ? 'hidden' : 'hidden sm:inline');

	let favourited = $state(false);
	let favouriteUri = $state('');
	let loading = $state(false);
	let scopeMissing = $state(false);

	$effect(() => {
		favourited = !!collection.viewer?.favourite;
		favouriteUri = collection.viewer?.favourite ?? '';
	});

	async function toggle() {
		if (!collection.cid || loading) return;
		loading = true;
		try {
			if (favourited) {
				if (favouriteUri && (await unfavouriteCollection(favouriteUri))) {
					favourited = false;
					favouriteUri = '';
					onChange?.(false);
					toast.success(`Removed '${collection.name}' from favourites`);
				}
			} else {
				const out = await favouriteCollection(collection.uri, collection.cid);
				if (out.status === 'ok') {
					favourited = true;
					favouriteUri = out.uri;
					onChange?.(true);
					toast.success(`Added '${collection.name}' to favourites`);
				} else if (out.status === 'scope-missing') {
					scopeMissing = true;
				}
			}
		} finally {
			loading = false;
		}
	}
</script>

<Toggle
	variant="outline"
	size="sm"
	class="shrink-0 self-start"
	aria-label={favourited ? 'Remove from favourites' : 'Add to favourites'}
	disabled={loading || !collection.cid}
	bind:pressed={() => favourited, () => toggle()}
>
	<Star class="size-4 {favourited ? 'fill-current' : ''}" />
	<span class={labelClass}>{favourited ? 'Favourite' : 'Add to favourites'}</span>
</Toggle>

<FollowScopeDialog
	bind:open={scopeMissing}
	description="To favourite collections, Currents needs permission to create favourite records on your AT Protocol account. You'll be redirected to re-authorize — it only takes a moment."
/>
