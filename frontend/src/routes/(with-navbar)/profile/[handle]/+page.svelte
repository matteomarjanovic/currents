<script lang="ts">
	import { page } from '$app/state';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import ProfileHeader from '$lib/components/profile-header.svelte';
	import CollectionCard from '$lib/components/collection-card.svelte';
	import type { ActorProfileView, CollectionView } from '$lib/types';

	let profile = $state<ActorProfileView | null>(null);
	let collections = $state<CollectionView[]>([]);
	let loading = $state(true);
	let notFound = $state(false);

	$effect(() => {
		const handle = page.params.handle ?? '';
		loading = true;
		notFound = false;
		profile = null;
		collections = [];

		Promise.all([
			fetch(
				`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.actor.getProfile?actor=${encodeURIComponent(handle)}`,
				{ credentials: 'include' }
			),
			fetch(
				`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getActorCollections?actor=${encodeURIComponent(handle)}&limit=50`,
				{ credentials: 'include' }
			)
		])
			.then(async ([pRes, cRes]) => {
				if (!pRes.ok) {
					notFound = true;
					return;
				}
				profile = await pRes.json();
				if (cRes.ok) {
					const data = await cRes.json();
					collections = data.collections ?? [];
				}
			})
			.catch(() => {
				notFound = true;
			})
			.finally(() => {
				loading = false;
			});
	});
</script>

<div class="mx-auto max-w-5xl">
	{#if loading}
		<Skeleton class="h-40 w-full rounded-xl sm:h-56" />
		<div class="-mt-10 flex items-end gap-4 sm:-mt-12">
			<Skeleton class="size-24 rounded-full sm:size-28" />
			<div class="flex-1 space-y-2 pb-2">
				<Skeleton class="h-6 w-48" />
				<Skeleton class="h-4 w-32" />
			</div>
		</div>
		<div class="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
			{#each Array(4) as _, i (i)}
				<div>
					<Skeleton class="aspect-square w-full rounded-lg" />
					<Skeleton class="mt-2 h-4 w-24" />
				</div>
			{/each}
		</div>
	{:else if notFound || !profile}
		<div class="py-24 text-center">
			<h1 class="text-lg font-medium text-foreground">Profile not found</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				We couldn't find a user for <span class="font-mono">@{page.params.handle}</span>.
			</p>
		</div>
	{:else}
		<ProfileHeader {profile} />

		<h2 class="mb-4 text-lg font-semibold text-foreground">Collections</h2>
		{#if collections.length === 0}
			<div class="py-12 text-center text-sm text-muted-foreground">No collections yet.</div>
		{:else}
			<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
				{#each collections as c (c.uri)}
					<CollectionCard collection={c} />
				{/each}
			</div>
		{/if}
	{/if}
</div>
