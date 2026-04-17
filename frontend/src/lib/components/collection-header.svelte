<script lang="ts">
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { buttonVariants } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import MoreHorizontal from '@lucide/svelte/icons/more-horizontal';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import type { CollectionView } from '$lib/types';

	interface Props {
		collection: CollectionView;
		isOwner: boolean;
		onEdit: () => void;
		onDelete: () => void;
	}

	let { collection, isOwner, onEdit, onDelete }: Props = $props();
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
			{#if collection.description}
				<p class="mt-3 whitespace-pre-wrap text-sm text-foreground">{collection.description}</p>
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
					<DropdownMenu.Separator />
					<DropdownMenu.Item onclick={onDelete} class="text-destructive focus:text-destructive">
						<Trash2 class="size-4" />
						Delete
					</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		{/if}
	</div>

	<Separator class="mt-6" />
</section>
