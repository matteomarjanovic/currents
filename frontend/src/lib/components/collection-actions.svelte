<script lang="ts">
	import type { Snippet } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { apiFetch } from '$lib/api';
	import {
		addCollection,
		collections,
		removeCollection,
		setCollectionPinned,
		updateCollection
	} from '$lib/stores/collections.svelte';
	import type { CollectionView } from '$lib/types';
	import * as ContextMenu from '$lib/components/ui/context-menu';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { buttonVariants } from '$lib/components/ui/button';
	import CollectionEditDialog from '$lib/components/collection-edit-dialog.svelte';
	import CollectionCreateDialog from '$lib/components/collection-create-dialog.svelte';
	import MoreHorizontal from '@lucide/svelte/icons/more-horizontal';
	import Pencil from '@lucide/svelte/icons/pencil';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import FolderInput from '@lucide/svelte/icons/folder-input';
	import Folder from '@lucide/svelte/icons/folder';
	import Library from '@lucide/svelte/icons/library';
	import Pin from '@lucide/svelte/icons/pin';
	import Trash2 from '@lucide/svelte/icons/trash-2';

	interface Props {
		collection: CollectionView;
		// `dropdown` renders a "…" trigger button; `context` wraps `children` in a
		// right-click/long-press trigger.
		variant?: 'dropdown' | 'context';
		children?: Snippet;
		// Called after the collection's record is deleted, so the caller can navigate
		// away if it was the one being viewed.
		onDeleted?: () => void;
	}

	let { collection, variant = 'dropdown', children, onDeleted }: Props = $props();

	// bits-ui's ContextMenu and DropdownMenu expose the same part API, so one
	// snippet renders both menus — same trick as the organize canvas tiles.
	const dropdownMenu = DropdownMenu as unknown as typeof ContextMenu;

	let editOpen = $state(false);
	let createSectionOpen = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	const rkey = $derived(collection.uri.split('/').pop() ?? '');
	const isSection = $derived(!!collection.parentUri);
	let sections = $derived(collections.items.filter((c) => c.parentUri === collection.uri));
	// Single level of nesting: a collection that already holds sections can't
	// become one itself.
	let canMove = $derived(sections.length === 0);
	// Root collections other than this one, and not the parent it already sits in.
	let moveTargets = $derived(
		collections.items
			.filter((c) => !c.parentUri && c.uri !== collection.uri && c.uri !== collection.parentUri)
			.sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
	);
	// getActorCollections already rolls the sections' saves into the parent's
	// count, so the delete warning reads it straight — never sum the sections in.
	let saveCount = $derived(collection.saveCount ?? 0);
	let pinned = $derived(!!collection.viewer?.pinned);

	async function togglePinned() {
		const next = !pinned;
		if (!(await setCollectionPinned(collection.uri, next))) {
			toast.error(`Couldn't ${next ? 'pin' : 'unpin'} '${collection.name}'`);
		}
	}

	async function move(targetUri: string, targetName: string) {
		const res = await apiFetch(`/api/collection/${rkey}`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				name: collection.name,
				description: collection.description ?? '',
				parent: targetUri
			})
		});
		if (!res.ok) {
			toast.error(`Couldn't move "${collection.name}" (${res.status}).`);
			return;
		}
		updateCollection(collection.uri, { parentUri: targetUri || undefined });
		toast.success(
			targetUri
				? `Moved "${collection.name}" into "${targetName}"`
				: `Moved "${collection.name}" to the top level`
		);
	}

	function onSaved(update: { name: string; description: string }) {
		updateCollection(collection.uri, {
			name: update.name,
			description: update.description || undefined
		});
	}

	function onSectionCreated(section: CollectionView) {
		addCollection(section);
	}

	async function confirmDelete() {
		deleting = true;
		try {
			const res = await apiFetch(`/api/collection/${rkey}`, { method: 'DELETE' });
			if (!res.ok) {
				toast.error(`Couldn't delete "${collection.name}" (${res.status}).`);
				return;
			}
			deleteOpen = false;
			// The cascade removes the sections' records too; drop them from the store
			// so the tree doesn't keep orphans around until the next load.
			for (const s of sections) removeCollection(s.uri);
			removeCollection(collection.uri);
			toast.success(`Collection "${collection.name}" deleted`);
			onDeleted?.();
		} catch {
			toast.error('Network error. Please try again.');
		} finally {
			deleting = false;
		}
	}
</script>

{#snippet menuItems(Menu: typeof ContextMenu)}
	<Menu.Item onSelect={togglePinned}>
		<Pin class={pinned ? 'fill-current' : ''} />
		{pinned ? 'Unpin' : 'Pin'}
	</Menu.Item>
	<Menu.Item onSelect={() => (editOpen = true)}>
		<Pencil />
		Edit
	</Menu.Item>
	{#if !isSection}
		<Menu.Item onSelect={() => (createSectionOpen = true)}>
			<FolderPlus />
			Create section
		</Menu.Item>
	{/if}
	{#if canMove}
		<Menu.Sub>
			<Menu.SubTrigger class="gap-2.5">
				<FolderInput />
				Move into
			</Menu.SubTrigger>
			<!-- Scroll on an inner div so the SubContent's frosted background stays
			     fixed and covers every row (see the canvas tile menu). -->
			<Menu.SubContent class="w-56 overflow-hidden p-0">
				<div class="max-h-[50vh] overflow-y-auto p-1.5">
					{#if isSection}
						<Menu.Item onSelect={() => move('', '')}>
							<Library />
							My library
						</Menu.Item>
						{#if moveTargets.length > 0}
							<Menu.Separator />
						{/if}
					{/if}
					{#each moveTargets as target (target.uri)}
						<Menu.Item onSelect={() => move(target.uri, target.name)}>
							<Folder />
							<span class="truncate">{target.name}</span>
						</Menu.Item>
					{/each}
					{#if moveTargets.length === 0 && !isSection}
						<p class="px-3 py-2 text-xs text-muted-foreground">No other collections yet.</p>
					{/if}
				</div>
			</Menu.SubContent>
		</Menu.Sub>
	{:else}
		<Menu.Item disabled class="items-start">
			<FolderInput class="mt-0.5" />
			<span class="flex flex-col items-start gap-0.5">
				<span>Move into</span>
				<span class="text-xs">A collection with sections can't become one</span>
			</span>
		</Menu.Item>
	{/if}
	<Menu.Separator />
	<Menu.Item onSelect={() => (deleteOpen = true)} class="text-destructive focus:text-destructive">
		<Trash2 />
		Delete
	</Menu.Item>
{/snippet}

{#if variant === 'context'}
	<ContextMenu.Root>
		<ContextMenu.Trigger>
			{#snippet child({ props })}
				<div {...props}>{@render children?.()}</div>
			{/snippet}
		</ContextMenu.Trigger>
		<!-- overflow-*-visible so the "Move into" submenu isn't clipped by the
		     menu's default overflow-x-hidden/overflow-y-auto. -->
		<ContextMenu.Content class="w-60 overflow-x-visible overflow-y-visible">
			{@render menuItems(ContextMenu)}
		</ContextMenu.Content>
	</ContextMenu.Root>
{:else}
	<DropdownMenu.Root>
		<DropdownMenu.Trigger
			class="{buttonVariants({ variant: 'ghost', size: 'icon' })} shrink-0"
			aria-label="Collection options"
		>
			<MoreHorizontal class="size-5" />
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="end" class="w-60 overflow-x-visible overflow-y-visible">
			{@render menuItems(dropdownMenu)}
		</DropdownMenu.Content>
	</DropdownMenu.Root>
{/if}

<CollectionEditDialog bind:open={editOpen} {collection} {onSaved} />

{#if !isSection}
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
			<AlertDialog.Title>Delete "{collection.name}"?</AlertDialog.Title>
			<AlertDialog.Description>
				This deletes the collection{sections.length > 0
					? `, its ${sections.length} ${sections.length === 1 ? 'section' : 'sections'},`
					: ''} and all
				{saveCount}
				{saveCount === 1 ? 'save' : 'saves'} inside{sections.length > 0 ? ' them' : ''}. This cannot
				be undone.
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
