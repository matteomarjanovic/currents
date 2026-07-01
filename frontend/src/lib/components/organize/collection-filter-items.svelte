<script lang="ts">
	import type { SvelteSet } from 'svelte/reactivity';
	import * as Command from '$lib/components/ui/command';
	import type { CollectionView } from '$lib/types';
	import Folder from '@lucide/svelte/icons/folder';
	import Star from '@lucide/svelte/icons/star';
	import Check from '@lucide/svelte/icons/check';

	// Collection-filter rows for a Command list (own + favourited), each a multi-select
	// checkbox. Shared by the search command and the find-similar filter so they match.
	// Must be rendered inside a <Command.Root>/<Command.List>.
	let {
		collections,
		favourites,
		selected,
		onToggle,
		pinned
	}: {
		collections: CollectionView[];
		favourites: CollectionView[];
		selected: SvelteSet<string>;
		onToggle: (uri: string) => void;
		// Optional: collections to float to the top of their group (kept in name order
		// among themselves). A frozen snapshot owned by the caller, so the order only
		// changes when the caller recomputes it — not as `selected` changes.
		pinned?: SvelteSet<string>;
	} = $props();

	const byName = (a: CollectionView, b: CollectionView) =>
		a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
	function order(list: CollectionView[]) {
		const sorted = [...list].sort(byName);
		if (!pinned?.size) return sorted;
		return [...sorted.filter((c) => pinned.has(c.uri)), ...sorted.filter((c) => !pinned.has(c.uri))];
	}
	let ownSorted = $derived(order(collections));
	let favSorted = $derived(order(favourites));
</script>

{#snippet checkbox(uri: string)}
	<span
		class="flex size-4 shrink-0 items-center justify-center rounded-[4px] border {selected.has(uri)
			? 'border-primary bg-primary text-primary-foreground'
			: 'border-input'}"
	>
		{#if selected.has(uri)}<Check class="size-3.5" />{/if}
	</span>
{/snippet}

{#if ownSorted.length > 0}
	<Command.Group heading="Filter by collection">
		{#each ownSorted as c (c.uri)}
			<Command.Item value={c.uri} onSelect={() => onToggle(c.uri)}>
				{@render checkbox(c.uri)}
				<Folder class="text-muted-foreground" />
				<span class="truncate">{c.name}</span>
			</Command.Item>
		{/each}
	</Command.Group>
{/if}

{#if favSorted.length > 0}
	<Command.Group heading="Favourites from others">
		{#each favSorted as c (c.uri)}
			<Command.Item value={c.uri} onSelect={() => onToggle(c.uri)}>
				{@render checkbox(c.uri)}
				<Star class="text-muted-foreground" />
				<span class="truncate">{c.name}</span>
			</Command.Item>
		{/each}
	</Command.Group>
{/if}
