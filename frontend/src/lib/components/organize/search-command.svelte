<script lang="ts">
	import { untrack } from 'svelte';
	import { Command as CommandPrimitive } from 'bits-ui';
	import { page } from '$app/state';
	import { SvelteSet } from 'svelte/reactivity';
	import * as Command from '$lib/components/ui/command';
	import * as InputGroup from '$lib/components/ui/input-group';
	import { Button } from '$lib/components/ui/button';
	import { ColorPicker } from '$lib/components/ui/color-picker';
	import ColorPaletteIcon from '$lib/components/color-palette-icon.svelte';
	import ColorTrialNote from '$lib/components/color-trial-note.svelte';
	import {
		features,
		isFeatureSeen,
		markFeatureSeen,
		FEATURE_COLOR_SEARCH
	} from '$lib/stores/features.svelte';
	import type { CollectionView } from '$lib/types';
	import CollectionFilterItems from '$lib/components/organize/collection-filter-items.svelte';
	import SearchIcon from '@lucide/svelte/icons/search';
	import FolderSearchIcon from '@lucide/svelte/icons/folder-search';
	import PaletteIcon from '@lucide/svelte/icons/palette';

	let {
		open = $bindable(false),
		collections,
		favourites,
		initial = [],
		initialText = '',
		canSearch,
		onSearch,
		onColorSearch
	}: {
		open?: boolean;
		collections: CollectionView[];
		favourites: CollectionView[];
		initial?: string[];
		// The active text query, so reopening during a hybrid search restores it.
		initialText?: string;
		// Supporter gate: resolves whether searching is allowed; the caller shows
		// its upgrade dialog on a false, so a rejection just closes the command.
		canSearch?: () => Promise<boolean>;
		onSearch: (query: string, collections: string[]) => void;
		// Color search (optionally with text = hybrid). Scoped via the header chip
		// afterwards, so it doesn't carry the command's collection selection.
		onColorSearch?: (hex: string, text?: string) => void;
	} = $props();

	const PRESETS = [
		'#e63946',
		'#f4a261',
		'#e9c46a',
		'#2a9d8f',
		'#457b9d',
		'#7b2cbf',
		'#ff70a6',
		'#111111',
		'#8d99ae',
		'#f8f9fa'
	];

	let query = $state('');
	let colorMode = $state(false);
	let color = $state('#e63946');

	// One-time "new" dot on the color-search toggle, cleared the moment the
	// color panel opens — by the toggle, or by reopening on a color search.
	let showColorNew = $derived(features.loaded && !isFeatureSeen(FEATURE_COLOR_SEARCH));
	$effect(() => {
		if (colorMode) void markFeatureSeen(FEATURE_COLOR_SEARCH);
	});
	// Collection URIs the search is narrowed to (empty = whole library).
	let selected = new SvelteSet<string>();
	// Collections pinned to the top of the list: a snapshot of the selection taken each
	// time the dialog opens, frozen for that session. Selecting/deselecting while open
	// doesn't reorder; reopening recomputes it, so a since-deselected collection drops
	// back to its normal place.
	let pinned = new SvelteSet<string>();

	let scopeLabel = $derived(
		selected.size > 0 ? ` in ${selected.size} collection${selected.size > 1 ? 's' : ''}` : ''
	);

	// The selection persists across opens (so a prior search's collections stay pinned on
	// reopen); only reseed from the current view collection when it actually changes
	// (navigating collections), not on every open.
	let prevOpen = false;
	let prevInitialKey = '';
	$effect(() => {
		const isOpen = open;
		untrack(() => {
			if (isOpen && !prevOpen) {
				// Opening while a color search is active starts in color mode on that
				// color (with any hybrid text restored), so the picker reopens to adjust it.
				const activeColor = page.url.searchParams.get('color');
				colorMode = !!activeColor;
				if (activeColor) color = '#' + activeColor;
				query = activeColor ? initialText : '';
				gateChecked = false;
				const key = initial.join(' ');
				if (key !== prevInitialKey) {
					selected.clear();
					for (const uri of initial) selected.add(uri);
					prevInitialKey = key;
				}
				pinned.clear();
				for (const uri of selected) pinned.add(uri);
			}
			prevOpen = isOpen;
		});
	});

	// Run the supporter gate once per open, on the first typed character; when
	// rejected, close the command (the caller's upgrade dialog takes over).
	let gateChecked = false;
	$effect(() => {
		if (!open || !canSearch || !query.trim()) return;
		untrack(() => {
			if (gateChecked) return;
			gateChecked = true;
			void canSearch().then((ok) => {
				if (!ok) open = false;
			});
		});
	});

	function toggle(uri: string) {
		if (selected.has(uri)) selected.delete(uri);
		else selected.add(uri);
	}

	async function submit() {
		const q = query.trim();
		if (!q) return;
		// Re-check on submit: typing may have raced the async gate above.
		if (canSearch && !(await canSearch())) return;
		onSearch(q, [...selected]);
		open = false;
	}

	function submitColor() {
		onColorSearch?.(color, query.trim() || undefined);
		open = false;
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			if (colorMode) submitColor();
			else submit();
		}
	}
</script>

<Command.Dialog
	bind:open
	shouldFilter={false}
	title="Search your library"
	description="Search your saved images by text or color, optionally narrowed to specific collections."
	class={colorMode ? 'top-1/2 -translate-y-1/2' : ''}
>
	<div class="p-1 pb-0">
		<InputGroup.Root class="h-9 bg-input/50">
			<CommandPrimitive.Input value={query}>
				{#snippet child({ props })}
					<InputGroup.Input
						{...props}
						bind:value={query}
						onkeydown={onKeydown}
						placeholder={colorMode ? 'Describe it too (optional)…' : 'Search your images…'}
					/>
				{/snippet}
			</CommandPrimitive.Input>
			<InputGroup.Addon>
				<SearchIcon class="size-4 shrink-0 opacity-50" />
			</InputGroup.Addon>
			<InputGroup.Addon align="inline-end">
				<InputGroup.Button
					size="icon-xs"
					class="relative"
					aria-label="Search by color"
					aria-pressed={colorMode}
					onclick={() => (colorMode = !colorMode)}
				>
					<ColorPaletteIcon filled={colorMode} />
					{#if showColorNew}
						<span
							class="absolute -top-0.5 -right-0.5 inline-flex h-2 w-2 rounded-full bg-red-500 ring-2 ring-background"
							aria-label="New feature available"
						></span>
					{/if}
				</InputGroup.Button>
			</InputGroup.Addon>
		</InputGroup.Root>
	</div>

	{#if colorMode}
		<div class="flex max-h-[75dvh] flex-col items-center gap-4 overflow-y-auto p-3">
			<ColorPicker bind:value={color} formats={['hex']} class="w-full max-w-[22rem]" />
			<div class="flex flex-wrap justify-center gap-2">
				{#each PRESETS as preset (preset)}
					<button
						type="button"
						aria-label="Pick {preset}"
						class="size-7 rounded-full border shadow-sm transition-transform hover:scale-110"
						style:background-color={preset}
						onclick={() => (color = preset)}
					></button>
				{/each}
			</div>
			<Button class="w-full" onclick={submitColor}>
				<PaletteIcon />
				{query.trim() ? `Search “${query.trim()}” in this color` : 'Search this color'}
			</Button>
			<ColorTrialNote />
		</div>
	{:else}
		<Command.List>
			<Command.Group>
				<Command.Item value="__search__" disabled={!query.trim()} onSelect={submit}>
					<FolderSearchIcon />
					<span class="truncate">
						Search {query.trim() ? `for “${query.trim()}”` : 'your library'}{scopeLabel}
					</span>
				</Command.Item>
			</Command.Group>

			<CollectionFilterItems {collections} {favourites} {selected} {pinned} onToggle={toggle} />
		</Command.List>
	{/if}
</Command.Dialog>
