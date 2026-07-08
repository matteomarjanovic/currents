<script lang="ts">
	import { untrack } from 'svelte';
	import { Command as CommandPrimitive } from 'bits-ui';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import * as Command from '$lib/components/ui/command';
	import * as InputGroup from '$lib/components/ui/input-group';
	import { Button } from '$lib/components/ui/button';
	import { ColorPicker } from '$lib/components/ui/color-picker';
	import ColorPaletteIcon from '$lib/components/color-palette-icon.svelte';
	import { requireSupporter } from '$lib/stores/supporter.svelte';
	import SearchIcon from '@lucide/svelte/icons/search';
	import ImageIcon from '@lucide/svelte/icons/image';
	import Folder from '@lucide/svelte/icons/folder';
	import UsersIcon from '@lucide/svelte/icons/users';
	import PaletteIcon from '@lucide/svelte/icons/palette';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const TYPES = [
		{ value: 'saves', label: 'images', icon: ImageIcon },
		{ value: 'collections', label: 'collections', icon: Folder },
		{ value: 'users', label: 'users', icon: UsersIcon }
	] as const;

	// A curated spread across the hue wheel plus neutrals, for one-tap picking.
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
	let trimmed = $derived(query.trim());

	let colorMode = $state(false);
	let color = $state('#e63946');

	// Reseed from the current search page each time the dialog opens: refining an
	// active text search starts from its query, and opening on a color-search page
	// starts already in color mode on that color.
	let prevOpen = false;
	$effect(() => {
		const isOpen = open;
		untrack(() => {
			if (isOpen && !prevOpen) {
				if (page.params.type === 'color') {
					colorMode = true;
					query = '';
					if (page.params.query) color = '#' + page.params.query;
				} else {
					colorMode = false;
					query = page.params.query ?? '';
				}
			}
			prevOpen = isOpen;
		});
	});

	function submit(type: (typeof TYPES)[number]['value']) {
		if (!trimmed) return;
		goto(
			resolve('/(with-navbar)/search/[type]/[query]', {
				type,
				query: encodeURIComponent(trimmed)
			})
		);
		open = false;
	}

	async function submitColor() {
		const hex = color.replace('#', '').toLowerCase();
		const go = () =>
			goto(resolve('/(with-navbar)/search/[type]/[query]', { type: 'color', query: hex }));
		open = false;
		// Gate before navigating; on a completed checkout the paywall resumes `go`.
		if (await requireSupporter(go)) go();
	}
</script>

<Command.Dialog
	bind:open
	shouldFilter={false}
	title="Search"
	description="Search images, collections, users, or by color."
	class={colorMode ? 'top-1/2 -translate-y-1/2' : ''}
>
	<div class="p-1 pb-0">
		<InputGroup.Root class="bg-input/50 h-9">
			<CommandPrimitive.Input value={query}>
				{#snippet child({ props })}
					<InputGroup.Input
						{...props}
						bind:value={query}
						disabled={colorMode}
						placeholder={colorMode ? 'Search by color…' : 'Search…'}
					/>
				{/snippet}
			</CommandPrimitive.Input>
			<InputGroup.Addon>
				<SearchIcon class="size-4 shrink-0 opacity-50" />
			</InputGroup.Addon>
			<InputGroup.Addon align="inline-end">
				<InputGroup.Button
					size="icon-xs"
					aria-label="Search by color"
					aria-pressed={colorMode}
					onclick={() => {
						colorMode = !colorMode;
						// Text and color are mutually exclusive for now; drop any typed
						// query so the disabled input shows the "Search by color…" hint.
						if (colorMode) query = '';
					}}
				>
					<ColorPaletteIcon filled={colorMode} />
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
				Search this color
			</Button>
		</div>
	{:else}
		<Command.List>
			<Command.Group>
				{#each TYPES as t (t.value)}
					<Command.Item value={t.value} disabled={!trimmed} onSelect={() => submit(t.value)}>
						<t.icon />
						<span class="truncate">
							Search {t.label}{trimmed ? ` for “${trimmed}”` : ''}
						</span>
					</Command.Item>
				{/each}
			</Command.Group>
		</Command.List>
	{/if}
</Command.Dialog>
