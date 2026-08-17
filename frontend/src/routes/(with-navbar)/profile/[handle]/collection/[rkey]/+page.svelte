<script lang="ts">
	import { untrack, onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { removeCollection } from '$lib/stores/collections.svelte';
	import { onSaveRemoved } from '$lib/stores/save-events.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import CollectionHeader from '$lib/components/collection-header.svelte';
	import CollectionCard from '$lib/components/collection-card.svelte';
	import CollectionEditDialog from '$lib/components/collection-edit-dialog.svelte';
	import CollectionCreateDialog from '$lib/components/collection-create-dialog.svelte';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import type { CollectionView } from '$lib/types';
	import { bunnyImageUrl } from '$lib/image-url';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const collectionUri = $derived(data.collectionUri);

	let collection = $state<CollectionView | null>(untrack(() => data.collection));
	let notFound = $state(untrack(() => !data.collection && !data.loadError));
	let loadError = $state(untrack(() => !!data.loadError));

	let editOpen = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);
	let createSectionOpen = $state(false);

	let children = $state<CollectionView[]>(untrack(() => data.children));
	let childrenLoaded = $state(true);
	let parent = $state<CollectionView | null>(null);
	let parentFetchedFor = '';

	$effect(() => {
		const pUri = collection?.parentUri ?? '';
		if (pUri === parentFetchedFor) return;
		parentFetchedFor = pUri;
		untrack(() => {
			parent = null;
		});
		if (!pUri) return;
		apiFetch(
			`/xrpc/is.currents.feed.getCollectionSaves?collection=${encodeURIComponent(pUri)}&limit=1`
		)
			.then((r) => (r.ok ? r.json() : null))
			.then((d) => {
				if (d?.collection) parent = d.collection;
			})
			.catch(() => {});
	});

	const isOwner = $derived(
		!!auth.user && !!collection?.author && auth.user.did === collection.author.did
	);

	const scroll = useInfiniteScroll(
		async (cursor) => {
			const params = new URLSearchParams({
				collection: collectionUri,
				limit: '50'
			});
			if (cursor) params.set('cursor', cursor);
			const res = await apiFetch(`/xrpc/is.currents.feed.getCollectionSaves?${params}`);
			if (res.status === 404) {
				notFound = true;
				return { items: [], cursor: undefined };
			}
			if (!res.ok) {
				loadError = true;
				throw new Error(`HTTP ${res.status}`);
			}
			const next = await res.json();
			if (next.collection) collection = next.collection;
			return { items: next.saves ?? [], cursor: next.cursor };
		},
		undefined,
		untrack(() => ({ items: data.saves, cursor: data.cursor }))
	);

	$effect(() => {
		const next = data;
		untrack(() => {
			collection = next.collection;
			notFound = !next.collection && !next.loadError;
			loadError = !!next.loadError;
			children = next.children;
			childrenLoaded = true;
			parent = null;
			parentFetchedFor = '';
			scroll.reset({ items: next.saves, cursor: next.cursor });
		});
	});

	// Refresh the universal load once after browser auth is known so viewer-specific favourite
	// and save state replaces the public server-rendered response.
	let viewerHydrated = false;
	$effect(() => {
		if (!auth.user || viewerHydrated) return;
		viewerHydrated = true;
		void invalidateAll();
	});

	// When an image is unsaved from this collection (e.g. via the save-detail overlay), drop it
	// from the grid immediately instead of leaving a stale entry until the next refetch.
	onMount(() =>
		onSaveRemoved(({ saveUri, collectionUri: removedFrom }) => {
			if (removedFrom === collectionUri) scroll.removeItem(saveUri);
		})
	);

	let sentinel: HTMLDivElement = $state(undefined!);
	$effect(() => {
		if (!sentinel) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) scroll.loadMore();
			},
			{ rootMargin: '400px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});

	function onEditClick() {
		editOpen = true;
	}

	function onDeleteClick() {
		deleteOpen = true;
	}

	function onSaved(update: { name: string; description: string }) {
		if (collection) {
			collection = { ...collection, name: update.name, description: update.description };
		}
	}

	function onSectionCreated(section: CollectionView) {
		children = [...children, section];
	}

	async function confirmDelete() {
		if (!collection) return;
		const rkey = collection.uri.split('/').pop();
		if (!rkey) return;
		deleting = true;
		try {
			const res = await apiFetch(`/api/collection/${rkey}`, {
				method: 'DELETE'
			});
			if (!res.ok) {
				deleting = false;
				return;
			}
			deleteOpen = false;
			removeCollection(collection.uri);
			toast.success(`Collection "${collection.name}" deleted`);
			const handle = auth.user?.handle ?? '';
			await goto(handle ? `/profile/${handle}` : '/', {
				replaceState: true,
				invalidateAll: true
			});
		} catch {
			deleting = false;
		}
	}

	function collectionHref(c: CollectionView) {
		const rkey = c.uri.split('/').pop() ?? '';
		const handle = c.author?.handle ?? c.uri.split('/')[2];
		return `/profile/${handle}/collection/${rkey}`;
	}

	const pageTitle = $derived((collection?.name || 'Collection') + ' · Currents');
	const description = $derived(
		(
			collection?.description?.trim() ||
			`${collection?.saveCount ?? 0} saves curated by @${collection?.author?.handle ?? data.collectionUri.split('/')[2]}.`
		).slice(0, 200)
	);
	const ogImage = $derived(
		collection?.previews?.[0]?.url
			? bunnyImageUrl(collection.previews[0].url, { width: 1200, quality: 85 })
			: ''
	);
	const canonical = $derived(page.url.origin + page.url.pathname);
</script>

<svelte:head>
	<title>{pageTitle}</title>
	<link rel="canonical" href={canonical} />
	<meta name="description" content={description} />
	<meta property="og:type" content="website" />
	<meta property="og:url" content={canonical} />
	<meta property="og:title" content={pageTitle} />
	<meta property="og:description" content={description} />
	{#if ogImage}
		<meta property="og:image" content={ogImage} />
		<meta name="twitter:image" content={ogImage} />
	{/if}
	<meta name="twitter:card" content={ogImage ? 'summary_large_image' : 'summary'} />
	<meta name="twitter:title" content={pageTitle} />
	<meta name="twitter:description" content={description} />
</svelte:head>

{#if notFound}
	<div class="mx-auto max-w-5xl">
		<div class="py-24 text-center">
			<h1 class="text-lg font-medium text-foreground">Collection not found</h1>
			<p class="mt-1 text-sm text-muted-foreground">This collection may have been deleted.</p>
		</div>
	</div>
{:else if loadError && !collection}
	<div class="mx-auto max-w-5xl">
		<div class="py-24 text-center">
			<h1 class="text-lg font-medium text-foreground">Something went wrong</h1>
			<p class="mt-1 text-sm text-muted-foreground">We couldn't load this collection.</p>
			<Button
				class="mt-4"
				variant="outline"
				onclick={() => {
					loadError = false;
					scroll.reset();
					scroll.loadMore();
				}}
			>
				Try again
			</Button>
		</div>
	</div>
{:else if !collection || !childrenLoaded}
	<div class="mx-auto mb-6 max-w-5xl space-y-3 px-1">
		<Skeleton class="h-8 w-64" />
		<Skeleton class="h-4 w-32" />
		<Skeleton class="h-4 w-full max-w-md" />
	</div>
	<MasonryGrid items={[]} loading={true} />
{:else}
	<div class="mx-auto max-w-5xl">
		<Button variant="ghost" size="sm" class="mb-1 -ml-2" onclick={() => history.back()}>
			<ArrowLeft />
			Back
		</Button>
		{#if parent}
			<span class="mb-1 ml-1 block text-sm text-muted-foreground">
				Section of
				<a href={collectionHref(parent)} class="text-foreground">
					{parent.name}
				</a>
			</span>
		{/if}
		<CollectionHeader
			{collection}
			{isOwner}
			onEdit={onEditClick}
			onDelete={onDeleteClick}
			onCreateSection={isOwner && !collection.parentUri
				? () => (createSectionOpen = true)
				: undefined}
		/>
	</div>

	{#if children.length > 0}
		<h2 class="mt-6 mb-3 text-base font-semibold text-foreground">Sections</h2>
		<div
			class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6"
		>
			{#each children as child (child.uri)}
				<CollectionCard collection={child} />
			{/each}
		</div>
		<h2 class="mt-8 mb-3 text-base font-semibold text-foreground">Saves</h2>
	{/if}

	{#if scroll.items.length === 0 && !scroll.loading && !scroll.hasMore}
		<div class="py-12 text-center text-sm text-muted-foreground">No saves yet.</div>
	{:else}
		<MasonryGrid items={scroll.items} loading={scroll.loading} loadMore={scroll.loadMore} />
		{#if scroll.hasMore}
			<div bind:this={sentinel} class="h-1"></div>
		{/if}
	{/if}

	{#if isOwner}
		<CollectionEditDialog bind:open={editOpen} {collection} {onSaved} />

		{#if !collection.parentUri}
			<CollectionCreateDialog
				bind:open={createSectionOpen}
				parent={collection.uri}
				parentName={collection.name}
				onCreated={onSectionCreated}
			/>
		{/if}

		<AlertDialog.Root bind:open={deleteOpen}>
			<AlertDialog.Content>
				<AlertDialog.Header>
					<AlertDialog.Title>Delete this collection?</AlertDialog.Title>
					<AlertDialog.Description>
						This will also remove
						{collection.saveCount ?? 0}
						{collection.saveCount === 1 ? 'save' : 'saves'}
						from your account. This cannot be undone.
					</AlertDialog.Description>
				</AlertDialog.Header>
				<AlertDialog.Footer>
					<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
					<AlertDialog.Action
						onclick={confirmDelete}
						disabled={deleting}
						class="text-destructive-foreground bg-destructive hover:bg-destructive/90"
					>
						{deleting ? 'Deleting…' : 'Delete'}
					</AlertDialog.Action>
				</AlertDialog.Footer>
			</AlertDialog.Content>
		</AlertDialog.Root>
	{/if}
{/if}
