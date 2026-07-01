<script lang="ts">
	import { untrack, type Snippet } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { apiFetch } from '$lib/api';
	import { getImageContent, type CollectionView, type SaveView } from '$lib/types';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		collections,
		setLastUsedCollection,
		addCollection
	} from '$lib/stores/collections.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { emitSaveRemoved } from '$lib/stores/save-events.svelte';
	import { RATE_LIMIT_MESSAGE } from '$lib/rate-limit';
	import { Button, type ButtonVariant } from '$lib/components/ui/button';
	import { Toggle } from '$lib/components/ui/toggle';
	import * as Popover from '$lib/components/ui/popover';
	import * as Drawer from '$lib/components/ui/drawer';
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import Plus from '@lucide/svelte/icons/plus';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import User from '@lucide/svelte/icons/user';
	import CollectionCreateDialog from '$lib/components/collection-create-dialog.svelte';
	import SaveToast from '$lib/components/save-toast.svelte';

	interface Props {
		item?: SaveView;
		// `inline` renders only the collection list (no trigger/wrapper) — for embedding
		// in a context-menu submenu or a custom drawer.
		variant?: 'popover' | 'drawer' | 'inline';
		// Style of the popover trigger button. Defaults to the translucent `glass`
		// (good over imagery, e.g. card tiles); pass a solid variant on plain
		// backgrounds where glass would blend in (e.g. the save-detail sidebar).
		triggerVariant?: ButtonVariant;
		// Custom trigger, replacing the default button (+ Save toggle). Receives the
		// popover/drawer trigger props to spread onto a focusable element.
		trigger?: Snippet<[{ props: Record<string, unknown> }]>;
		onOpenChange?: (open: boolean) => void;
		selectedUri?: string;
		onSelect?: (uri: string) => void;
		onSavesChange?: (saves: { collectionUri: string; saveUri: string }[]) => void;
	}

	let {
		item,
		variant = 'popover',
		triggerVariant = 'glass',
		trigger,
		onOpenChange,
		selectedUri,
		onSelect,
		onSavesChange
	}: Props = $props();

	let pickerMode = $derived(!item);

	let localSaves = $state<{ collectionUri: string; saveUri: string }[]>(
		untrack(() => (item?.viewer?.saves ? [...item.viewer.saves] : []))
	);
	let syncedItemUri = $state<string | null>(null);

	let userSelectedUri = $state<string | null>(null);
	let selectedCollectionUri = $derived(
		pickerMode
			? selectedUri
			: (userSelectedUri ??
					toTopLevel(localSaves[0]?.collectionUri ?? collections.lastUsedUri ?? ''))
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
	let createParent = $state<CollectionView | null>(null);
	// When set, the list shows this collection's sections instead of the roots.
	let drillParent = $state<CollectionView | null>(null);
	$effect(() => {
		onOpenChange?.(open);
	});
	// Always start back at the top level when the picker reopens.
	$effect(() => {
		if (!open) drillParent = null;
	});

	const OPTIMISTIC_URI = 'optimistic';
	// Saves with no collection ("unsorted") use the empty string as their collection
	// URI — matching the server's representation — so the same save/unsave machinery
	// keyed on collection URI works for the profile pseudo-destination.
	const UNSORTED_URI = '';

	function isSavedIn(uri: string): string | null {
		return localSaves.find((s) => s.collectionUri === uri)?.saveUri ?? null;
	}

	// Resolve a collection URI to its top-level parent (sub-collections → parent).
	function toTopLevel(uri: string): string {
		const c = collections.items.find((x) => x.uri === uri);
		return c?.parentUri ? c.parentUri : uri;
	}

	// Most recent activity first: newest of {created, last save}.
	function activityTs(c: CollectionView): number {
		const saved = c.lastSavedAt ? Date.parse(c.lastSavedAt) : 0;
		const created = c.createdAt ? Date.parse(c.createdAt) : 0;
		return Math.max(saved, created);
	}
	function byRecentSave(a: CollectionView, b: CollectionView): number {
		return activityTs(b) - activityTs(a);
	}

	let childrenByParent = $derived.by(() => {
		const m = new Map<string, CollectionView[]>();
		for (const c of collections.items) {
			if (c.parentUri) {
				const arr = m.get(c.parentUri) ?? [];
				arr.push(c);
				m.set(c.parentUri, arr);
			}
		}
		return m;
	});
	let rootCollections = $derived(collections.items.filter((c) => !c.parentUri).sort(byRecentSave));
	let drillSections = $derived(
		drillParent ? [...(childrenByParent.get(drillParent.uri) ?? [])].sort(byRecentSave) : []
	);

	function sectionCount(uri: string): number {
		return childrenByParent.get(uri)?.length ?? 0;
	}

	// A root is "saved" if the item is in it directly or in any of its sections.
	function isSavedInTree(root: CollectionView): boolean {
		if (isSavedIn(root.uri)) return true;
		return (childrenByParent.get(root.uri) ?? []).some((c) => !!isSavedIn(c.uri));
	}

	function openCreate(parent: CollectionView | null) {
		createParent = parent;
		open = false;
		createOpen = true;
	}

	let selectedName = $derived(
		selectedCollectionUri === UNSORTED_URI
			? 'Profile (unsorted)'
			: (collections.items.find((c) => c.uri === selectedCollectionUri)?.name ??
					'Select collection')
	);

	function isSavedInSelected() {
		return isSavedIn(selectedCollectionUri ?? '');
	}

	let anySaved = $derived(localSaves.length > 0);

	async function save(collectionUri: string) {
		if (!item) return;
		localSaves = [...localSaves, { collectionUri, saveUri: OPTIMISTIC_URI }];
		onSavesChange?.(localSaves);
		if (collectionUri !== UNSORTED_URI) setLastUsedCollection(collectionUri);
		try {
			const res = await apiFetch(`/resave`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ saveUri: item.uri, collectionUri })
			});
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
				} else if (res.status === 429) {
					toast.error(RATE_LIMIT_MESSAGE);
				}
				throw new Error(`save: ${res.status}`);
			}
			const data = await res.json();
			localSaves = localSaves.map((s) =>
				s.collectionUri === collectionUri && s.saveUri === OPTIMISTIC_URI
					? { collectionUri, saveUri: data.uri }
					: s
			);
			onSavesChange?.(localSaves);
			const collectionName =
				collectionUri === UNSORTED_URI
					? 'your profile'
					: (collections.items.find((c) => c.uri === collectionUri)?.name ?? 'collection');
			toast(SaveToast, {
				componentProps: {
					imageUrl: getImageContent(item)?.imageUrl,
					collectionName
				}
			});
		} catch (e) {
			console.error('save failed', e);
			localSaves = localSaves.filter(
				(s) => !(s.collectionUri === collectionUri && s.saveUri === OPTIMISTIC_URI)
			);
			onSavesChange?.(localSaves);
		}
	}

	async function unsave(saveUri: string, collectionUri: string) {
		const prev = localSaves;
		localSaves = localSaves.filter((s) => s.saveUri !== saveUri);
		onSavesChange?.(localSaves);
		try {
			const rkey = saveUri.split('/').pop()!;
			const res = await apiFetch(`/save/${rkey}`, {
				method: 'DELETE'
			});
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
				}
				throw new Error(`unsave: ${res.status}`);
			}
			const collectionName =
				collectionUri === UNSORTED_URI
					? 'your profile'
					: (collections.items.find((c) => c.uri === collectionUri)?.name ?? 'collection');
			toast.success(`Removed from ${collectionName}`);
			// Let an open collection / unsorted grid drop this image without a refetch.
			emitSaveRemoved({ saveUri, collectionUri });
		} catch (e) {
			console.error('unsave failed', e);
			localSaves = prev;
			onSavesChange?.(localSaves);
		}
	}

	function toggleCollection(uri: string) {
		if (pickerMode) {
			onSelect?.(uri);
			if (uri !== UNSORTED_URI) setLastUsedCollection(uri);
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
		// '' is a valid target (the profile / unsorted destination), so don't bail on it.
		const uri = selectedCollectionUri ?? '';
		const existing = isSavedIn(uri);
		if (existing) {
			unsave(existing, uri);
		} else {
			save(uri);
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

{#snippet preview(col: CollectionView)}
	{#if col.previews?.[0]}
		<img
			src={col.previews[0].url}
			alt=""
			loading="lazy"
			class="size-9 shrink-0 rounded-md object-cover"
		/>
	{:else}
		<div class="size-9 shrink-0 rounded-md bg-muted"></div>
	{/if}
{/snippet}

<!-- A leaf row: clicking it saves (or selects) the collection. -->
{#snippet saveRow(col: CollectionView, subtitle: string, bold: boolean)}
	<button
		class="flex w-full items-center gap-2.5 rounded-2xl px-2 py-1.5 text-sm hover:bg-foreground/10"
		onclick={() => toggleCollection(col.uri)}
	>
		{@render preview(col)}
		<span class="flex flex-1 flex-col items-start truncate">
			<span class="truncate {bold ? 'font-medium' : ''}">{col.name}</span>
			<span class="text-xs text-muted-foreground">{subtitle}</span>
		</span>
		{#if isSavedIn(col.uri)}
			<Check class="size-4 shrink-0" />
		{/if}
	</button>
{/snippet}

<!-- A collection with sections: clicking it drills into its sections. -->
{#snippet navRow(root: CollectionView)}
	{@const n = sectionCount(root.uri)}
	<button
		class="flex w-full items-center gap-2.5 rounded-2xl px-2 py-1.5 text-sm hover:bg-foreground/10"
		onclick={() => (drillParent = root)}
	>
		{@render preview(root)}
		<span class="flex flex-1 flex-col items-start truncate">
			<span class="truncate">{root.name}</span>
			<span class="text-xs text-muted-foreground">
				Public • {n}
				{n === 1 ? 'section' : 'sections'}
			</span>
		</span>
		{#if isSavedInTree(root)}
			<Check class="size-4 shrink-0 text-muted-foreground" />
		{/if}
		<ChevronRight class="size-4 shrink-0 text-muted-foreground" />
	</button>
{/snippet}

{#snippet collectionList()}
	{#if drillParent}
		{@const dp = drillParent}
		<button
			class="flex w-full items-center gap-1.5 rounded-2xl px-2 py-1.5 text-sm font-medium hover:bg-foreground/10"
			onclick={() => (drillParent = null)}
		>
			<ChevronLeft class="size-4 shrink-0" />
			<span class="truncate">All collections</span>
		</button>
		{@render saveRow(dp, 'Whole collection', true)}
		<!-- Sections -->
		<span class="block px-2 pt-3 text-xs text-muted-foreground">Sections</span>
		<button
			class="flex w-full items-center gap-2.5 rounded-2xl px-3 py-2 text-sm hover:bg-foreground/10"
			onclick={() => openCreate(dp)}
		>
			<FolderPlus class="size-4 shrink-0" />
			<span class="truncate">Create section</span>
		</button>
		{#each drillSections as sec (sec.uri)}
			{@render saveRow(sec, 'Public', false)}
		{/each}
	{:else}
		<!-- Save directly to the profile, with no collection ("unsorted"). -->
		<button
			class="flex w-full items-center gap-2.5 rounded-2xl px-2 py-1.5 text-sm hover:bg-foreground/10"
			onclick={() => toggleCollection(UNSORTED_URI)}
		>
			<span class="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted">
				<User class="size-4 text-muted-foreground" />
			</span>
			<span class="flex flex-1 flex-col items-start truncate">
				<span class="truncate">Profile</span>
				<span class="text-xs text-muted-foreground">Save without a collection</span>
			</span>
			{#if isSavedIn(UNSORTED_URI)}
				<Check class="size-4 shrink-0" />
			{/if}
		</button>
		<button
			class="flex w-full items-center gap-2.5 rounded-2xl px-2 py-1.5 text-sm hover:bg-foreground/10"
			onclick={() => openCreate(null)}
		>
			<span class="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted">
				<Plus class="size-4 text-muted-foreground" />
			</span>
			<span class="flex flex-1 flex-col items-start truncate">
				<span class="truncate">Create new collection</span>
				<span class="text-xs text-muted-foreground">Group your saves</span>
			</span>
		</button>
		{#each rootCollections as root (root.uri)}
			{#if sectionCount(root.uri) > 0}
				{@render navRow(root)}
			{:else}
				{@render saveRow(root, 'Public', false)}
			{/if}
		{/each}
	{/if}
{/snippet}

{#if variant === 'popover'}
	<div class="flex items-center gap-1.5">
		<Popover.Root bind:open>
			<Popover.Trigger>
				{#snippet child({ props })}
					{#if trigger}
						{@render trigger({ props })}
					{:else}
						<Button
							{...props}
							variant={triggerVariant}
							size="default"
							class="min-w-0 flex-1 justify-between truncate"
						>
							<span class="truncate">{selectedName}</span>
							<ChevronDown class="ml-1 size-3 shrink-0" />
						</Button>
					{/if}
				{/snippet}
			</Popover.Trigger>
			<Popover.Content
				align="start"
				class="scrollbar-hide max-h-[40vh] gap-0 overflow-y-auto bg-popover/70 p-1.5 backdrop-blur-2xl backdrop-saturate-150"
			>
				{@render collectionList()}
			</Popover.Content>
		</Popover.Root>

		{#if !pickerMode && !trigger}
			<Toggle
				size="default"
				pressed={!!isSavedInSelected()}
				onPressedChange={handleButtonClick}
				class="border border-transparent bg-primary text-primary-foreground hover:bg-primary/80 aria-pressed:bg-secondary aria-pressed:text-secondary-foreground aria-pressed:hover:bg-secondary/80"
			>
				{isSavedInSelected() ? 'Saved' : 'Save'}
			</Toggle>
		{/if}
	</div>
{:else if variant === 'inline'}
	{@render collectionList()}
{:else}
	<Drawer.Root bind:open>
		<Drawer.Trigger>
			{#snippet child({ props })}
				{#if trigger}
					{@render trigger({ props })}
				{:else if pickerMode}
					<Button {...props} variant="secondary" class="w-full justify-between">
						<span class="truncate">{selectedName}</span>
						<ChevronDown class="ml-1 size-3 shrink-0" />
					</Button>
				{:else}
					<Button
						{...props}
						variant={anySaved ? 'secondary' : 'default'}
						size="lg"
						class="w-full rounded-full"
					>
						{anySaved ? 'Saved' : 'Save'}
					</Button>
				{/if}
			{/snippet}
		</Drawer.Trigger>
		<Drawer.Content>
			<Drawer.Header>
				<Drawer.Title>
					{#if drillParent}
						{drillParent.name}
					{:else if pickerMode}
						Pick a collection
					{:else}
						Save to collection
					{/if}
				</Drawer.Title>
				<Drawer.Description>
					{#if drillParent}
						Choose a section, or save to the whole collection.
					{:else if pickerMode}
						Choose a collection for the uploads.
					{:else}
						Pick one or more collections.
					{/if}
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

<CollectionCreateDialog
	bind:open={createOpen}
	parent={createParent?.uri}
	parentName={createParent?.name}
	onCreated={handleCreated}
/>
