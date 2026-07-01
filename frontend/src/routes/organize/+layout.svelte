<script lang="ts">
	import '../layout.css';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { ModeWatcher } from 'mode-watcher';
	import { Toaster } from '$lib/components/ui/sonner';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections, loadCollections } from '$lib/stores/collections.svelte';
	import { favouriteCollections, loadFavouriteCollections } from '$lib/stores/favourites.svelte';
	import { apiFetch } from '$lib/api';
	import { isNative } from '$lib/platform';

	let { children } = $props();

	// Organize mode lives outside the (with-navbar) group, so it runs its own
	// auth check (mirroring that layout) and gates rendering on a logged-in user.
	onMount(async () => {
		if (!auth.checked) {
			try {
				const res = await apiFetch('/api/me');
				auth.user = res.ok ? await res.json() : null;
			} catch {
				auth.user = null;
			}
			auth.checked = true;
		}
		if (!auth.user) {
			goto(isNative() ? '/' : '/login');
			return;
		}
		if (!collections.loaded) void loadCollections(auth.user.did);
		if (!favouriteCollections.loaded) void loadFavouriteCollections(auth.user.did);
	});
</script>

<ModeWatcher />
{#if auth.checked && auth.user}
	{@render children()}
{/if}
<Toaster />
