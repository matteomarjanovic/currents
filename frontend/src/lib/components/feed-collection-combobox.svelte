<script lang="ts">
	import * as Command from '$lib/components/ui/command';
	import * as Popover from '$lib/components/ui/popover';
	import { Button } from '$lib/components/ui/button';
	import { collections } from '$lib/stores/collections.svelte';
	import {
		feedPreferences,
		feedPreferencesLoaded,
		setCollectionExcluded
	} from '$lib/stores/feed-preferences.svelte';
	import type { CollectionView } from '$lib/types';
	import CheckIcon from '@lucide/svelte/icons/check';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';

	let open = $state(false);
	let sorted = $derived(
		[...collections.items].sort((a, b) =>
			a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
		)
	);
	let namesByURI = $derived(new Map(collections.items.map((c) => [c.uri, c.name])));
	let excludedCount = $derived(feedPreferences.excludedCollections.length);

	function isExcluded(uri: string) {
		return feedPreferences.excludedCollections.includes(uri);
	}

	function toggle(collection: CollectionView) {
		setCollectionExcluded(collection.uri, !isExcluded(collection.uri));
	}
</script>

<Popover.Root bind:open>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button
				{...props}
				variant="outline"
				class="w-full justify-between"
				role="combobox"
				aria-expanded={open}
				aria-label="Choose collections to exclude from feed personalization"
				disabled={!feedPreferencesLoaded.value || !collections.loaded}
			>
				<span class="truncate">
					{#if !feedPreferencesLoaded.value || !collections.loaded}
						Loading…
					{:else if excludedCount === 0}
						Select collections…
					{:else if excludedCount === 1}
						1 collection excluded
					{:else}
						{excludedCount} collections excluded
					{/if}
				</span>
				<ChevronsUpDownIcon class="size-4 shrink-0 opacity-50" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content align="start" class="w-(--bits-popover-anchor-width) min-w-64 p-0">
		<Command.Root class="bg-transparent">
			<Command.Input placeholder="Search collections…" />
			<Command.List class="max-h-64">
				<Command.Empty>No collection found.</Command.Empty>
				<Command.Group value="collections">
					{#each sorted as collection (collection.uri)}
						{@const excluded = isExcluded(collection.uri)}
						<Command.Item
							value={`${collection.name} ${namesByURI.get(collection.parentUri ?? '') ?? ''} ${collection.uri}`}
							onSelect={() => toggle(collection)}
						>
							<CheckIcon class="size-4 {excluded ? 'opacity-100' : 'opacity-0'}" />
							<span class="min-w-0">
								<span class="block truncate">{collection.name}</span>
								{#if collection.parentUri}
									<span class="block truncate text-xs text-muted-foreground">
										Section in {namesByURI.get(collection.parentUri) ?? 'collection'}
									</span>
								{/if}
							</span>
						</Command.Item>
					{/each}
				</Command.Group>
			</Command.List>
		</Command.Root>
	</Popover.Content>
</Popover.Root>
