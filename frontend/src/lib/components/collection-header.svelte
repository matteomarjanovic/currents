<script lang="ts">
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { buttonVariants } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import MoreHorizontal from '@lucide/svelte/icons/more-horizontal';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import Tag from '@lucide/svelte/icons/tag';
	import type { CollectionView } from '$lib/types';
	import Star from '@lucide/svelte/icons/star';
	import FavouriteToggle from './favourite-toggle.svelte';
	import { auth } from '$lib/stores/auth.svelte';

	interface Props {
		collection: CollectionView;
		isOwner: boolean;
		onEdit: () => void;
		onDelete: () => void;
		// Provided only when a section can be created here (owned, root-level collection).
		onCreateSection?: () => void;
		// Provided only when the collection has labelable (own, non-resave) saves.
		onBulkLabel?: () => void;
	}

	let { collection, isOwner, onEdit, onDelete, onCreateSection, onBulkLabel }: Props = $props();

	// You can favourite only other people's collections, and only when signed in.
	// The count is shown here and updated optimistically by the toggle's onChange.
	const loggedIn = $derived(!!auth.user);
	let favouriteCount = $state(0);
	$effect(() => {
		favouriteCount = collection.favouriteCount ?? 0;
	});
</script>

<section class="mb-6">
	<div class="flex items-start justify-between gap-3 px-1">
		<div class="min-w-0 flex-1">
			<h1 class="truncate text-2xl font-semibold text-foreground">{collection.name}</h1>
			{#if collection.author}
				<a
					href={`/profile/${collection.author.handle}`}
					class="text-sm text-muted-foreground hover:text-foreground hover:underline"
				>
					@{collection.author.handle}
				</a>
			{/if}
			{#if collection.saveCount != null}
				<span class="ml-2 text-sm text-muted-foreground">
					· {collection.saveCount}
					{collection.saveCount === 1 ? 'save' : 'saves'}
				</span>
			{/if}
			{#if favouriteCount > 0}
				<span
					class="ml-2 inline-flex items-center gap-1 align-middle text-sm text-muted-foreground"
					title={`${favouriteCount} ${favouriteCount === 1 ? 'favourite' : 'favourites'}`}
				>
					<Star class="size-3.5 fill-current" />
					{favouriteCount}
				</span>
			{/if}
		</div>

		{#if isOwner}
			<DropdownMenu.Root>
				<DropdownMenu.Trigger
					class={buttonVariants({ variant: 'ghost', size: 'icon' })}
					aria-label="Collection options"
				>
					<MoreHorizontal class="size-5" />
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end" class="w-40">
					<DropdownMenu.Item onclick={onEdit}>
						<Pencil class="size-4" />
						Edit
					</DropdownMenu.Item>
					{#if onCreateSection}
						<DropdownMenu.Item onclick={onCreateSection}>
							<FolderPlus class="size-4" />
							Create section
						</DropdownMenu.Item>
					{/if}
					{#if onBulkLabel}
						<DropdownMenu.Item onclick={onBulkLabel}>
							<Tag class="size-4" />
							Apply labels to images
						</DropdownMenu.Item>
					{/if}
					<DropdownMenu.Separator />
					<DropdownMenu.Item onclick={onDelete} class="text-destructive focus:text-destructive">
						<Trash2 class="size-4" />
						Delete
					</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		{:else if loggedIn}
			<FavouriteToggle
				{collection}
				onChange={(fav) => (favouriteCount = Math.max(0, favouriteCount + (fav ? 1 : -1)))}
			/>
		{/if}
	</div>

	{#if collection.description}
		<p class="mt-3 px-1 text-sm wrap-anywhere whitespace-pre-wrap text-foreground">
			{collection.description}
		</p>
	{/if}

	<Separator class="mt-6" />
</section>
