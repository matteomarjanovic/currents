<script lang="ts">
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import CollectionHeader from '$lib/components/collection-header.svelte';
	import CollectionEditDialog from '$lib/components/collection-edit-dialog.svelte';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import type { CollectionView } from '$lib/types';

	const collectionUri = $derived(decodeURIComponent(page.params.uri ?? ''));

	let collection = $state<CollectionView | null>(null);
	let notFound = $state(false);
	let loadError = $state(false);

	let editOpen = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	const isOwner = $derived(
		!!auth.user && !!collection?.author && auth.user.did === collection.author.did
	);

	const scroll = useInfiniteScroll(async (cursor) => {
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

	async function confirmDelete() {
		if (!collection) return;
		const rkey = collection.uri.split('/').pop();
		if (!rkey) return;
		deleting = true;
		try {
			const res = await apiFetch(`/collection/${rkey}`, {
				method: 'DELETE'
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
</script>

<div class="mx-auto max-w-5xl">
	{#if notFound}
		<div class="py-24 text-center">
			<h1 class="text-lg font-medium text-foreground">Collection not found</h1>
			<p class="mt-1 text-sm text-muted-foreground">This collection may have been deleted.</p>
		</div>
	{:else if loadError && !collection}
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
	{:else if !collection}
		<div class="mb-6 space-y-3 px-1">
			<Skeleton class="h-8 w-64" />
			<Skeleton class="h-4 w-32" />
			<Skeleton class="h-4 w-full max-w-md" />
		</div>
		<MasonryGrid items={[]} loading={true} />
	{:else}
		<CollectionHeader
			{collection}
			{isOwner}
			onEdit={onEditClick}
			onDelete={onDeleteClick}
		/>

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
							class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						>
							{deleting ? 'Deleting…' : 'Delete'}
						</AlertDialog.Action>
					</AlertDialog.Footer>
				</AlertDialog.Content>
			</AlertDialog.Root>
		{/if}
	{/if}
</div>
