<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { auth } from '$lib/stores/auth.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import CollectionHeader from '$lib/components/collection-header.svelte';
	import CollectionCard from '$lib/components/collection-card.svelte';
	import CollectionEditDialog from '$lib/components/collection-edit-dialog.svelte';
	import CollectionCreateDialog from '$lib/components/collection-create-dialog.svelte';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import type { CollectionView } from '$lib/types';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const collectionUri = $derived(data.collectionUri);

	let collection = $state<CollectionView | null>(null);
	let notFound = $state(false);
	let loadError = $state(false);

	let editOpen = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);
	let createSectionOpen = $state(false);

	let children = $state<CollectionView[]>([]);
	let childrenLoaded = $state(false);
	let parent = $state<CollectionView | null>(null);
	let parentFetchedFor = '';

	$effect(() => {
		const uri = collectionUri;
		untrack(() => {
			children = [];
			childrenLoaded = false;
		});
		const did = uri.split('/')[2];
		if (!did) {
			childrenLoaded = true;
			return;
		}
		fetch(
			`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getActorCollections?actor=${encodeURIComponent(did)}&parent=${encodeURIComponent(uri)}&limit=100`,
			{ credentials: 'include' }
		)
			.then((r) => (r.ok ? r.json() : null))
			.then((d) => {
				if (d) children = d.collections ?? [];
			})
			.catch(() => {})
			.finally(() => {
				childrenLoaded = true;
			});
	});

	$effect(() => {
		const pUri = collection?.parentUri ?? '';
		if (pUri === parentFetchedFor) return;
		parentFetchedFor = pUri;
		untrack(() => {
			parent = null;
		});
		if (!pUri) return;
		fetch(
			`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getCollectionSaves?collection=${encodeURIComponent(pUri)}&limit=1`,
			{ credentials: 'include' }
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

	const scroll = useInfiniteScroll(async (cursor) => {
		const params = new URLSearchParams({
			collection: collectionUri,
			limit: '50'
		});
		if (cursor) params.set('cursor', cursor);
		const res = await fetch(
			`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getCollectionSaves?${params}`,
			{ credentials: 'include' }
		);
		if (res.status === 404) {
			notFound = true;
			return { items: [], cursor: undefined };
		}
		if (!res.ok) {
			loadError = true;
			throw new Error(`HTTP ${res.status}`);
		}
		const data = await res.json();
		if (data.collection) collection = data.collection;
		return { items: data.saves ?? [], cursor: data.cursor };
	});

	$effect(() => {
		void collectionUri;
		untrack(() => {
			collection = null;
			notFound = false;
			loadError = false;
			scroll.reset();
			scroll.loadMore();
		});
	});

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
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/collection/${rkey}`, {
				method: 'DELETE',
				credentials: 'include'
			});
			if (!res.ok) {
				deleting = false;
				return;
			}
			deleteOpen = false;
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
</script>

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
		{#if parent}
			<a
				href={collectionHref(parent)}
				class="mb-2 inline-block text-sm text-muted-foreground hover:text-foreground"
			>
				← {parent.name}
			</a>
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
		<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6">
			{#each children as child (child.uri)}
				<CollectionCard collection={child} />
			{/each}
		</div>
		<h2 class="mt-8 mb-3 text-base font-semibold text-foreground">Saves</h2>
	{/if}

	{#if scroll.items.length === 0 && !scroll.loading && !scroll.hasMore}
		<div class="py-12 text-center text-sm text-muted-foreground">No saves yet.</div>
	{:else}
		<MasonryGrid items={scroll.items} loading={scroll.loading} />
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
