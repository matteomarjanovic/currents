<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetch } from '$lib/api';
	import { requireSupporter } from '$lib/stores/supporter.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import CollectionCard from '$lib/components/collection-card.svelte';
	import * as Avatar from '$lib/components/ui/avatar';
	import SupporterBadge from '$lib/components/supporter-badge.svelte';
	import UserIcon from '@lucide/svelte/icons/user';
	import type { SaveView, CollectionView, ActorProfileView } from '$lib/types';

	type SearchType = 'saves' | 'collections' | 'users' | 'color';
	type Result = SaveView | CollectionView | ActorProfileView;

	const ENDPOINTS: Record<SearchType, { path: string; key: string }> = {
		saves: { path: 'is.currents.feed.searchSaves', key: 'saves' },
		collections: { path: 'is.currents.feed.searchCollections', key: 'collections' },
		users: { path: 'is.currents.actor.searchActors', key: 'actors' },
		color: { path: 'is.currents.feed.searchSavesByColor', key: 'saves' }
	};

	let searchType = $derived<SearchType>(
		(page.params.type as SearchType) in ENDPOINTS ? (page.params.type as SearchType) : 'saves'
	);

	// For color search the [query] param is the bare hex (no '#'). An optional ?q
	// turns it into a hybrid search: the color filters, the text orders.
	let colorHex = $derived('#' + (page.params.query ?? ''));
	let hybridText = $derived(searchType === 'color' ? (page.url.searchParams.get('q') ?? '') : '');

	// Which type the items currently in `search` belong to. The selector can flip
	// `searchType` (and thus the render branch) a frame before the list is reset and
	// reloaded — without this guard the new branch would render the previous type's
	// objects under the wrong cast and crash (e.g. a user has no `uri`).
	let loadedType = $state<SearchType>('saves');

	const search = useInfiniteScroll<Result>(
		async (cursor) => {
			const fetchedType = searchType;
			const q = page.params.query ?? '';
			const cfg = ENDPOINTS[fetchedType];
			const params = new SvelteURLSearchParams({ limit: '50' });
			if (fetchedType === 'color') {
				params.set('color', '#' + q);
				if (hybridText) params.set('q', hybridText);
			} else {
				params.set('q', q);
			}
			if (fetchedType === 'saves') params.set('excludeSaved', 'true');
			if (cursor) params.set('cursor', cursor);

			const res = await apiFetch(`/xrpc/${cfg.path}?${params}`);
			if (!res.ok) {
				// Deep-linked non-supporter (or logged-out) hitting the gated color
				// endpoint: raise the paywall, show nothing.
				if (res.status === 403) void requireSupporter();
				loadedType = fetchedType;
				return { items: [], cursor: undefined };
			}
			const data = await res.json();
			loadedType = fetchedType;
			return { items: data[cfg.key] ?? [], cursor: data.cursor };
		},
		(i) => ('did' in i ? i.did : i.uri)
	);

	let saves = $derived(
		loadedType === 'saves' || loadedType === 'color' ? (search.items as SaveView[]) : []
	);
	let collections = $derived(
		loadedType === 'collections' ? (search.items as CollectionView[]) : []
	);
	let actors = $derived(loadedType === 'users' ? (search.items as ActorProfileView[]) : []);

	const pageTitle = $derived(
		searchType === 'color'
			? (hybridText ? `${hybridText} · ${colorHex}` : colorHex) + ' · Color search'
			: page.params.query
				? page.params.query + ' · Search'
				: 'Search'
	);

	let sentinel: HTMLDivElement = $state(undefined!);

	$effect(() => {
		void page.params.query;
		void searchType;
		void hybridText;
		const timeout = setTimeout(() => {
			untrack(() => {
				search.reset();
				search.loadMore();
			});
		}, 300);
		return () => clearTimeout(timeout);
	});

	$effect(() => {
		if (!sentinel) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) search.loadMore();
			},
			{ rootMargin: '400px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});
</script>

<svelte:head>
	<title>{pageTitle} · Currents</title>
</svelte:head>

{#if searchType === 'color'}
	<div class="mb-4 flex items-center gap-2 text-sm text-muted-foreground">
		<span class="size-5 rounded-full border shadow-sm" style:background-color={colorHex}></span>
		<span class="font-mono uppercase">{colorHex}</span>
		{#if hybridText}
			<span>·</span>
			<span class="truncate">“{hybridText}”</span>
		{/if}
	</div>
	<MasonryGrid items={saves} loading={search.loading} />
	{#if !search.loading && loadedType === 'color' && saves.length === 0}
		<p class="mt-10 text-center text-sm text-muted-foreground">
			{#if hybridText}
				No “{hybridText}” images in this color. Try a broader color or drop the text.
			{:else}
				No images found for this color.
			{/if}
		</p>
	{/if}
{:else if searchType === 'saves'}
	<MasonryGrid items={saves} loading={search.loading} />
{:else if searchType === 'collections'}
	<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
		{#each collections as collection (collection.uri)}
			<CollectionCard {collection} />
		{/each}
	</div>
	{#if !search.loading && loadedType === 'collections' && collections.length === 0}
		<p class="mt-10 text-center text-sm text-muted-foreground">No collections found.</p>
	{/if}
{:else}
	<div class="mx-auto flex max-w-2xl flex-col gap-1">
		{#each actors as actor (actor.did)}
			<a
				href={resolve('/(with-navbar)/profile/[handle]', { handle: actor.handle })}
				class="flex items-center gap-3 rounded-lg p-2 transition-colors hover:bg-muted"
			>
				<Avatar.Root class="size-12">
					{#if actor.avatar}
						<Avatar.Image src={actor.avatar} alt={actor.displayName ?? actor.handle} />
					{/if}
					<Avatar.Fallback>
						<UserIcon class="size-5" />
					</Avatar.Fallback>
				</Avatar.Root>
				<div class="min-w-0">
					<div class="flex items-center gap-1.5 font-medium text-foreground">
						<span class="min-w-0 truncate">{actor.displayName ?? actor.handle}</span>
						{#if actor.supporter}
							<SupporterBadge class="size-4 shrink-0 text-foreground" />
						{/if}
					</div>
					<div class="truncate text-sm text-muted-foreground">@{actor.handle}</div>
					{#if actor.description}
						<div class="truncate text-sm text-muted-foreground">{actor.description}</div>
					{/if}
				</div>
			</a>
		{/each}
	</div>
	{#if !search.loading && loadedType === 'users' && actors.length === 0}
		<p class="mt-10 text-center text-sm text-muted-foreground">No users found.</p>
	{/if}
{/if}

{#if search.hasMore}
	<div bind:this={sentinel} class="h-1"></div>
{/if}
