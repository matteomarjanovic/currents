<script lang="ts">
	import { untrack } from 'svelte';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import type { CollectionView, SaveView } from '$lib/types';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		collections,
		setLastUsedCollection,
		addCollection
	} from '$lib/stores/collections.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Toggle } from '$lib/components/ui/toggle';
	import * as Popover from '$lib/components/ui/popover';
	import * as Drawer from '$lib/components/ui/drawer';
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import Plus from '@lucide/svelte/icons/plus';
	import CollectionCreateDialog from '$lib/components/collection-create-dialog.svelte';

	interface Props {
		item?: SaveView;
		variant?: 'popover' | 'drawer';
		onOpenChange?: (open: boolean) => void;
		selectedUri?: string;
		onSelect?: (uri: string) => void;
	}

	let { item, variant = 'popover', onOpenChange, selectedUri, onSelect }: Props = $props();

	let pickerMode = $derived(!item);

	let localSaves = $state<{ collectionUri: string; saveUri: string }[]>(
		untrack(() => (item?.viewer?.saves ? [...item.viewer.saves] : []))
	);
	let syncedItemUri = $state<string | null>(null);

	let userSelectedUri = $state<string | null>(null);
	let selectedCollectionUri = $derived(
		pickerMode
			? (selectedUri ?? '')
			: (userSelectedUri ?? localSaves[0]?.collectionUri ?? collections.lastUsedUri)
	);

	$effect(() => {
		if (!item) return;
		const nextItemUri = item.uri;
		localSaves = item.viewer?.saves ? [...item.viewer.saves] : [];
		if (syncedItemUri !== nextItemUri) {
			userSelectedUri = null;
		}
		syncedItemUri = nextItemUri;
	});

	let open = $state(false);
	let createOpen = $state(false);
	$effect(() => {
		onOpenChange?.(open);
	});

	const OPTIMISTIC_URI = 'optimistic';

	function isSavedIn(uri: string): string | null {
		return localSaves.find((s) => s.collectionUri === uri)?.saveUri ?? null;
	}

	let sortedCollections = $derived(
		[...collections.items].sort((a, b) => {
			const aSaved = isSavedIn(a.uri) ? 0 : 1;
			const bSaved = isSavedIn(b.uri) ? 0 : 1;
			return aSaved - bSaved;
		})
	);

	let selectedName = $derived(
		collections.items.find((c) => c.uri === selectedCollectionUri)?.name ?? 'Collection'
	);

	function isSavedInSelected() {
		return isSavedIn(selectedCollectionUri);
	}

	let anySaved = $derived(localSaves.length > 0);

	async function save(collectionUri: string) {
		if (!item) return;
		localSaves = [...localSaves, { collectionUri, saveUri: OPTIMISTIC_URI }];
		setLastUsedCollection(collectionUri);
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/resave`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ saveUri: item.uri, collectionUri }),
				credentials: 'include'
			});
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
				}
				throw new Error(`save: ${res.status}`);
			}
			const data = await res.json();
			localSaves = localSaves.map((s) =>
				s.collectionUri === collectionUri && s.saveUri === OPTIMISTIC_URI
					? { collectionUri, saveUri: data.uri }
					: s
			);
		} catch (e) {
			console.error('save failed', e);
			localSaves = localSaves.filter(
				(s) => !(s.collectionUri === collectionUri && s.saveUri === OPTIMISTIC_URI)
			);
		}
	}

	async function unsave(saveUri: string, collectionUri: string) {
		const prev = localSaves;
		localSaves = localSaves.filter((s) => s.saveUri !== saveUri);
		try {
			const rkey = saveUri.split('/').pop()!;
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/save/${rkey}`, {
				method: 'DELETE',
				credentials: 'include'
			});
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
				}
				throw new Error(`unsave: ${res.status}`);
			}
		} catch (e) {
			console.error('unsave failed', e);
			localSaves = prev;
		}
	}

	function toggleCollection(uri: string) {
		if (pickerMode) {
			onSelect?.(uri);
			setLastUsedCollection(uri);
			open = false;
			return;
		}
		userSelectedUri = uri;
		const existing = isSavedIn(uri);
		if (existing) {
			unsave(existing, uri);
		} else {
			save(uri);
		}
		if (variant === 'popover') open = false;
	}

	function handleButtonClick() {
		if (!selectedCollectionUri) return;
		const existing = isSavedIn(selectedCollectionUri);
		if (existing) {
			unsave(existing, selectedCollectionUri);
		} else {
			save(selectedCollectionUri);
		}
	}

	function handleCreated(collection: CollectionView) {
		addCollection(collection);
		setLastUsedCollection(collection.uri);
		if (pickerMode) {
			onSelect?.(collection.uri);
		} else {
			userSelectedUri = collection.uri;
			save(collection.uri);
		}
	}
</script>

{#snippet collectionList()}
	<button
		class="flex w-full items-center gap-2.5 rounded-2xl px-3 py-2 text-sm hover:bg-foreground/10"
		onclick={() => {
			open = false;
			createOpen = true;
		}}
	>
		<Plus class="size-4 shrink-0" />
		<span class="truncate">Create new collection</span>
	</button>
	{#each sortedCollections as col (col.uri)}
		<button
			class="flex w-full items-center gap-2.5 rounded-2xl px-2 py-1.5 text-sm hover:bg-foreground/10"
			onclick={() => toggleCollection(col.uri)}
		>
			{#if col.previewImages?.[0]}
				<img
					src={col.previewImages[0]}
					alt=""
					loading="lazy"
					class="size-9 shrink-0 rounded-md object-cover"
				/>
			{:else}
				<div class="size-9 shrink-0 rounded-md bg-muted"></div>
			{/if}
			<span class="flex flex-1 flex-col items-start truncate">
				<span class="truncate">{col.name}</span>
				<span class="text-xs text-muted-foreground">Public</span>
			</span>
			{#if isSavedIn(col.uri)}
				<Check class="size-4 shrink-0" />
			{/if}
		</button>
	{/each}
{/snippet}

{#if variant === 'popover'}
	<div class="flex items-center gap-1.5">
		<Popover.Root bind:open>
			<Popover.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						variant="secondary"
						size="sm"
						class="min-w-0 flex-1 justify-between truncate"
					>
						<span class="truncate">{selectedName}</span>
						<ChevronDown class="ml-1 size-3 shrink-0" />
					</Button>
				{/snippet}
			</Popover.Trigger>
			<Popover.Content
				align="start"
				class="scrollbar-hide max-h-[40vh] gap-0 overflow-y-auto bg-popover/70 p-1.5 backdrop-blur-2xl backdrop-saturate-150"
			>
				{@render collectionList()}
			</Popover.Content>
		</Popover.Root>

		{#if !pickerMode}
			<Toggle
				size="sm"
				pressed={!!isSavedInSelected()}
				onPressedChange={handleButtonClick}
				disabled={!selectedCollectionUri}
				class="border border-transparent bg-primary text-primary-foreground hover:bg-primary/80 aria-pressed:bg-secondary aria-pressed:text-secondary-foreground aria-pressed:hover:bg-secondary/80"
			>
				{isSavedInSelected() ? 'Saved' : 'Save'}
			</Toggle>
		{/if}
	</div>
{:else}
	<Drawer.Root bind:open>
		<Drawer.Trigger>
			{#snippet child({ props })}
				{#if pickerMode}
					<Button {...props} variant="secondary" class="w-full justify-between">
						<span class="truncate">{selectedName}</span>
						<ChevronDown class="ml-1 size-3 shrink-0" />
					</Button>
				{:else}
					<Button {...props} variant={anySaved ? 'secondary' : 'default'} class="w-full">
						{anySaved ? 'Saved' : 'Save'}
					</Button>
				{/if}
			{/snippet}
		</Drawer.Trigger>
		<Drawer.Content>
			<Drawer.Header>
				<Drawer.Title>{pickerMode ? 'Pick a collection' : 'Save to collection'}</Drawer.Title>
				<Drawer.Description>
					{pickerMode ? 'Choose a collection for the uploads.' : 'Pick one or more collections.'}
				</Drawer.Description>
			</Drawer.Header>
			<div class="max-h-[60vh] overflow-y-auto p-1.5">
				{@render collectionList()}
			</div>
			<Drawer.Footer>
				<Drawer.Close>
					{#snippet child({ props })}
						<Button {...props} variant="outline">Done</Button>
					{/snippet}
				</Drawer.Close>
			</Drawer.Footer>
		</Drawer.Content>
	</Drawer.Root>
{/if}

<CollectionCreateDialog bind:open={createOpen} onCreated={handleCreated} />
