<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetch } from '$lib/api';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Tabs from '$lib/components/ui/tabs';
	import { auth } from '$lib/stores/auth.svelte';
	import { deletedCollectionUris } from '$lib/stores/collections.svelte';
	import { onSaveRemoved } from '$lib/stores/save-events.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import ProfileHeader from '$lib/components/profile-header.svelte';
	import ProfileEditDialog from '$lib/components/profile-edit-dialog.svelte';
	import CollectionCard from '$lib/components/collection-card.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import type { ActorProfileView, CollectionView } from '$lib/types';

	let profile = $state<ActorProfileView | null>(null);
	let collections = $state<CollectionView[]>([]);
	let loading = $state(true);
	let notFound = $state(false);
	let editOpen = $state(false);
	let activeImport = $state(false);

	const isOwner = $derived(!!auth.user && !!profile && auth.user.did === profile.did);
	const pageTitle = $derived(profile?.displayName || '@' + (profile?.handle ?? page.params.handle));

	// Show only root collections as cards, most recent activity first:
	// newest of {created, last save}.
	const activityTs = (c: (typeof collections)[number]) =>
		Math.max(
			c.lastSavedAt ? Date.parse(c.lastSavedAt) : 0,
			c.createdAt ? Date.parse(c.createdAt) : 0
		);
	const roots = $derived(
		collections
			.filter((c) => !c.parentUri && !deletedCollectionUris.has(c.uri))
			.sort((a, b) => activityTs(b) - activityTs(a))
	);
	const sectionCounts = $derived.by(() => {
		const m = new Map<string, number>();
		for (const c of collections) {
			if (c.parentUri) m.set(c.parentUri, (m.get(c.parentUri) ?? 0) + 1);
		}
		return m;
	});

	// Collections vs. Unsorted (saves in no collection — profile-only). Unsorted is
	// fetched lazily the first time its tab is opened.
	let activeTab = $state('collections');
	const unsorted = useInfiniteScroll(async (cursor) => {
		const handle = page.params.handle ?? '';
		const params = new URLSearchParams({ actor: handle, limit: '50' });
		if (cursor) params.set('cursor', cursor);
		const res = await apiFetch(`/xrpc/is.currents.feed.getUnsortedSaves?${params}`);
		if (!res.ok) return { items: [], cursor: undefined };
		const data = await res.json();
		return { items: data.saves ?? [], cursor: data.cursor };
	});

	// Drop an unsorted save from the grid when it's unsaved from the profile elsewhere
	// (UNSORTED_URI is the empty-string sentinel). Matches by the deleted record's uri.
	onMount(() =>
		onSaveRemoved(({ saveUri, collectionUri }) => {
			if (collectionUri === '') unsorted.removeItem(saveUri);
		})
	);

	// Collections this user has favourited (public, like GitHub stars). Fetched
	// lazily the first time the Favourites tab is opened.
	const favourites = useInfiniteScroll<CollectionView>(async (cursor) => {
		const handle = page.params.handle ?? '';
		const params = new URLSearchParams({ actor: handle, limit: '30' });
		if (cursor) params.set('cursor', cursor);
		const res = await apiFetch(`/xrpc/is.currents.graph.getFavouriteCollections?${params}`);
		if (!res.ok) return { items: [], cursor: undefined };
		const data = await res.json();
		return { items: data.collections ?? [], cursor: data.cursor };
	});

	$effect(() => {
		const handle = page.params.handle ?? '';
		loading = true;
		notFound = false;
		profile = null;
		collections = [];
		activeTab = 'collections';
		unsorted.reset();
		favourites.reset();

		Promise.all([
			apiFetch(`/xrpc/is.currents.actor.getProfile?actor=${encodeURIComponent(handle)}`),
			apiFetch(
				`/xrpc/is.currents.feed.getActorCollections?actor=${encodeURIComponent(handle)}&limit=100`
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

	// On your own profile, surface an in-flight Pinterest import so you can jump
	// to its status (imports run server-side and can take a long time).
	$effect(() => {
		if (!isOwner) {
			activeImport = false;
			return;
		}
		void (async () => {
			try {
				const res = await apiFetch(`/api/import/active-session`);
				if (!res.ok) return;
				const data = await res.json();
				activeImport = !!data.sessionId;
			} catch {
				// best-effort; no banner on failure
			}
		})();
	});

	// Load the first page of unsorted saves / favourites the first time each tab is opened.
	$effect(() => {
		if (activeTab === 'unsorted' && unsorted.items.length === 0 && unsorted.hasMore) {
			unsorted.loadMore();
		}
	});
	$effect(() => {
		if (activeTab === 'favourites' && favourites.items.length === 0 && favourites.hasMore) {
			favourites.loadMore();
		}
	});

	let sentinel: HTMLDivElement | undefined = $state(undefined);
	$effect(() => {
		if (!sentinel) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) unsorted.loadMore();
			},
			{ rootMargin: '400px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});

	let favSentinel: HTMLDivElement | undefined = $state(undefined);
	$effect(() => {
		if (!favSentinel) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) favourites.loadMore();
			},
			{ rootMargin: '400px' }
		);
		observer.observe(favSentinel);
		return () => observer.disconnect();
	});

	function onProfileSaved(updated: ActorProfileView) {
		profile = updated;
		if (auth.user && auth.user.did === updated.did) {
			auth.user = {
				...auth.user,
				handle: updated.handle,
				displayName: updated.displayName,
				avatar: updated.avatar
			};
		}
	}
</script>

<svelte:head>
	<title>{pageTitle} · Currents</title>
</svelte:head>

<div class="mx-auto max-w-5xl pb-16">
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
			{#each [0, 1, 2, 3] as i (i)}
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
		<ProfileHeader {profile} {isOwner} onEdit={() => (editOpen = true)} />

		{#if isOwner && activeImport}
			<a
				href="/import/pinterest"
				class="mb-6 flex items-center gap-2.5 rounded-lg border bg-muted/40 px-4 py-3 text-sm transition-colors hover:bg-muted"
			>
				<span class="relative flex size-2">
					<span
						class="absolute inline-flex size-full animate-ping rounded-full bg-primary opacity-75"
					></span>
					<span class="relative inline-flex size-2 rounded-full bg-primary"></span>
				</span>
				<span
					>A Pinterest import is in progress — <span class="font-medium underline">view status</span
					></span
				>
			</a>
		{/if}

		<Tabs.Root bind:value={activeTab}>
			<!-- Horizontal scroll so the tab strip never overflows the page on mobile.
			     pb/-mb pair keeps room for the line-variant underline (sits ~1px below the
			     list) which overflow-x would otherwise clip. -->
			<div
				class="-mb-1.5 overflow-x-auto pb-1.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
			>
				<Tabs.List variant="line" class="w-fit">
					<Tabs.Trigger value="collections">Collections</Tabs.Trigger>
					<Tabs.Trigger value="unsorted">Unsorted</Tabs.Trigger>
					<Tabs.Trigger value="favourites">Favourite collections</Tabs.Trigger>
				</Tabs.List>
			</div>

			<Tabs.Content value="collections" class="mt-4">
				{#if roots.length === 0}
					<div class="py-12 text-center text-sm text-muted-foreground">No collections yet.</div>
				{:else}
					<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
						{#each roots as c (c.uri)}
							<CollectionCard collection={c} sectionCount={sectionCounts.get(c.uri) ?? 0} />
						{/each}
					</div>
				{/if}
			</Tabs.Content>

			<Tabs.Content value="unsorted" class="mt-4">
				{#if unsorted.items.length === 0 && !unsorted.loading && !unsorted.hasMore}
					<div class="py-12 text-center text-sm text-muted-foreground">No unsorted saves yet.</div>
				{:else}
					<MasonryGrid items={unsorted.items} loading={unsorted.loading} />
					{#if unsorted.hasMore}
						<div bind:this={sentinel} class="h-1"></div>
					{/if}
				{/if}
			</Tabs.Content>

			<Tabs.Content value="favourites" class="mt-4">
				{#if favourites.items.length === 0 && !favourites.loading && !favourites.hasMore}
					<div class="py-12 text-center text-sm text-muted-foreground">
						No favourite collections yet.
					</div>
				{:else}
					<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
						{#each favourites.items as c (c.uri)}
							<CollectionCard collection={c} sectionCount={0} />
						{/each}
					</div>
					{#if favourites.hasMore}
						<div bind:this={favSentinel} class="h-1"></div>
					{/if}
				{/if}
			</Tabs.Content>
		</Tabs.Root>

		{#if isOwner}
			<ProfileEditDialog bind:open={editOpen} {profile} onSaved={onProfileSaved} />
		{/if}
	{/if}
</div>
