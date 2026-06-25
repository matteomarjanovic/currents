<script lang="ts">
	import { untrack, onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { removeCollection } from '$lib/stores/collections.svelte';
	import { onSaveRemoved } from '$lib/stores/save-events.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import SelectableSaveGrid from '$lib/components/selectable-save-grid.svelte';
	import CollectionHeader from '$lib/components/collection-header.svelte';
	import CollectionCard from '$lib/components/collection-card.svelte';
	import CollectionEditDialog from '$lib/components/collection-edit-dialog.svelte';
	import CollectionCreateDialog from '$lib/components/collection-create-dialog.svelte';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
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
		apiFetch(
			`/xrpc/is.currents.feed.getActorCollections?actor=${encodeURIComponent(did)}&parent=${encodeURIComponent(uri)}&limit=100`
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
			const res = await apiFetch(`/collection/${rkey}`, {
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

	// ── Bulk self-labeling (owner) ────────────────────────────────────────────
	const SELF_LABEL_OPTIONS = [
		{ val: 'porn', label: 'Porn' },
		{ val: 'sexual', label: 'Sexual' },
		{ val: 'nudity', label: 'Nudity' },
		{ val: 'graphic-media', label: 'Graphic' },
		{ val: 'currents-ai-generated', label: 'AI-generated' }
	];
	let selectionMode = $state(false);
	let selectedUris = $state<Set<string>>(new Set());
	let bulkLabels = $state<Set<string>>(new Set());
	let applying = $state(false);
	let bulkResult = $state<{ applied: number; skipped: number; failed: number } | null>(null);

	function enterSelection() {
		selectionMode = true;
		selectedUris = new Set();
		bulkLabels = new Set();
		bulkResult = null;
	}
	function exitSelection() {
		selectionMode = false;
		selectedUris = new Set();
		bulkLabels = new Set();
	}
	function toggleSelect(uri: string) {
		const next = new Set(selectedUris);
		if (next.has(uri)) next.delete(uri);
		else next.add(uri);
		selectedUris = next;
	}
	function selectAllLoaded() {
		selectedUris = new Set(scroll.items.filter((i) => !i.resaveOf).map((i) => i.uri));
	}
	function toggleBulkLabel(val: string) {
		const next = new Set(bulkLabels);
		if (next.has(val)) next.delete(val);
		else next.add(val);
		bulkLabels = next;
	}

	async function applyBulkLabels() {
		const rkeys = [...selectedUris].map((u) => u.split('/').pop()).filter(Boolean) as string[];
		const labels = [...bulkLabels];
		if (rkeys.length === 0 || labels.length === 0) return;
		applying = true;
		bulkResult = null;
		try {
			const res = await apiFetch(`/save/labels/bulk`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ rkeys, labels })
			});
			if (!res.ok) {
				bulkResult = { applied: 0, skipped: 0, failed: rkeys.length };
				return;
			}
			bulkResult = await res.json();
			selectedUris = new Set();
			bulkLabels = new Set();
		} catch {
			bulkResult = { applied: 0, skipped: 0, failed: rkeys.length };
		} finally {
			applying = false;
		}
	}
</script>

<svelte:head>
	<title>{(collection?.name || 'Collection') + ' · Currents'}</title>
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
			onBulkLabel={isOwner ? enterSelection : undefined}
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
		{#if selectionMode}
			<SelectableSaveGrid items={scroll.items} selected={selectedUris} onToggle={toggleSelect} />
		{:else}
			<MasonryGrid items={scroll.items} loading={scroll.loading} />
		{/if}
		{#if scroll.hasMore}
			<div bind:this={sentinel} class="h-1"></div>
		{/if}
		{#if selectionMode}
			<div class="h-44"></div>
		{/if}
	{/if}

	{#if selectionMode}
		<div
			class="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-popover/95 p-3 backdrop-blur-sm"
		>
			<div class="mx-auto flex max-w-5xl flex-col gap-3">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<div class="flex items-center gap-3 text-sm">
						<span class="font-medium">{selectedUris.size} selected</span>
						<button
							type="button"
							class="text-xs text-muted-foreground underline-offset-2 hover:underline"
							onclick={selectAllLoaded}
						>
							Select all loaded
						</button>
						{#if selectedUris.size > 0}
							<button
								type="button"
								class="text-xs text-muted-foreground underline-offset-2 hover:underline"
								onclick={() => (selectedUris = new Set())}
							>
								Clear
							</button>
						{/if}
					</div>
					<div class="flex items-center gap-2">
						<Button variant="ghost" size="sm" onclick={exitSelection} disabled={applying}>
							Cancel
						</Button>
						<Button
							size="sm"
							onclick={applyBulkLabels}
							disabled={applying || selectedUris.size === 0 || bulkLabels.size === 0}
						>
							{applying ? 'Applying…' : `Apply to ${selectedUris.size}`}
						</Button>
					</div>
				</div>
				<div class="flex flex-wrap items-center gap-1.5 text-xs">
					<span class="text-muted-foreground">Labels to add:</span>
					{#each SELF_LABEL_OPTIONS as opt (opt.val)}
						{@const active = bulkLabels.has(opt.val)}
						<button
							type="button"
							onclick={() => toggleBulkLabel(opt.val)}
							disabled={applying}
							class="rounded-full border px-2.5 py-1 transition-colors {active
								? 'border-foreground bg-foreground text-background'
								: 'border-border text-muted-foreground hover:bg-muted'}"
						>
							{opt.label}
						</button>
					{/each}
				</div>
				<p class="text-xs text-muted-foreground">
					Resaves can't be labeled and aren't selectable. Add-only — labels can't be removed here,
					and apply to every copy of each image.
				</p>
				{#if bulkResult}
					<p class="text-xs font-medium text-foreground">
						Applied to {bulkResult.applied}{bulkResult.skipped > 0
							? ` · ${bulkResult.skipped} skipped`
							: ''}{bulkResult.failed > 0 ? ` · ${bulkResult.failed} failed` : ''}. Labels may take
						a moment to appear.
					</p>
				{/if}
			</div>
		</div>
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
