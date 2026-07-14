<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import {
		features,
		isFeatureSeen,
		markFeatureSeen,
		FEATURE_ORGANIZE_MODE
	} from '$lib/stores/features.svelte';
	import Compass from '@lucide/svelte/icons/compass';
	import Folders from '@lucide/svelte/icons/folders';
	import Check from '@lucide/svelte/icons/check';
	import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down';

	let {
		mode,
		variant = 'floating',
		side = 'bottom',
		anchor,
		class: className = ''
	}: {
		mode: 'explore' | 'organize';
		variant?: 'floating' | 'sidebar' | 'icon';
		side?: 'top' | 'bottom';
		// Anchor the menu to another element (centered) instead of the trigger.
		anchor?: HTMLElement;
		class?: string;
	} = $props();

	const MODES = [
		{
			value: 'explore',
			label: 'Explore',
			description: 'Discover new images',
			icon: Compass,
			open: () => goto(resolve('/(with-navbar)/explore'))
		},
		{
			value: 'organize',
			label: 'Organize',
			description: 'Curate your library',
			icon: Folders,
			open: () => goto(resolve('/organize'))
		}
	] as const;

	let current = $derived(MODES.find((m) => m.value === mode) ?? MODES[0]);

	// One-time "new" announcement for organize mode: a red dot on the explore
	// switcher and a "New" badge on the Organize row, cleared once the user
	// reaches organize mode by any route (this component renders in the organize
	// sidebar, so the effect fires there).
	let showOrganizeNew = $derived(features.loaded && !isFeatureSeen(FEATURE_ORGANIZE_MODE));
	let showOrganizeDot = $derived(mode === 'explore' && showOrganizeNew);
	$effect(() => {
		if (mode === 'organize') void markFeatureSeen(FEATURE_ORGANIZE_MODE);
	});
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			{#if variant === 'icon'}
				<button
					{...props}
					type="button"
					aria-label="Switch mode"
					class="relative flex size-9 shrink-0 items-center justify-center rounded-full text-foreground transition-colors outline-none select-none hover:bg-muted aria-expanded:bg-muted {className}"
				>
					<current.icon class="size-4" />
					{#if showOrganizeDot}
						<span
							class="absolute -top-0.5 -right-0.5 inline-flex h-2.5 w-2.5 rounded-full bg-red-500 ring-2 ring-background"
							aria-label="New feature available"
						></span>
					{/if}
				</button>
			{:else if variant === 'floating'}
				<button
					{...props}
					type="button"
					class="relative flex h-11 shrink-0 items-center gap-1.5 rounded-full border border-transparent bg-primary-foreground/80 bg-clip-padding px-3.5 text-sm font-medium text-foreground shadow-sm backdrop-blur-sm transition-colors outline-none select-none hover:bg-primary-foreground aria-expanded:bg-primary-foreground {className}"
				>
					<current.icon class="size-4" />
					{current.label}
					<ChevronsUpDown class="size-3.5 text-muted-foreground" />
					{#if showOrganizeDot}
						<span
							class="absolute top-0 right-0.5 inline-flex h-2.5 w-2.5 rounded-full bg-red-500 ring-2 ring-background"
							aria-label="New feature available"
						></span>
					{/if}
				</button>
			{:else}
				<button
					{...props}
					type="button"
					class="flex w-full items-center gap-2 rounded-md p-2 text-left text-sm text-sidebar-foreground transition-colors outline-none hover:bg-sidebar-accent hover:text-sidebar-accent-foreground data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground {className}"
				>
					<span
						class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground"
					>
						<current.icon class="size-4" />
					</span>
					<span class="grid flex-1 text-left leading-tight">
						<span class="truncate font-medium">{current.label}</span>
						<span class="truncate text-xs text-muted-foreground">Switch mode…</span>
					</span>
					<ChevronsUpDown class="size-4 text-muted-foreground" />
				</button>
			{/if}
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content
		{side}
		align={anchor ? 'center' : 'start'}
		customAnchor={anchor ?? null}
		class="w-52"
	>
		{#each MODES as m (m.value)}
			<DropdownMenu.Item onclick={() => m.value !== mode && m.open()}>
				<m.icon class="size-4" />
				<span class="grid flex-1 leading-tight">
					<span>{m.label}</span>
					<span class="text-xs text-muted-foreground">{m.description}</span>
				</span>
				{#if m.value === mode}
					<Check class="ml-auto size-4" />
				{:else if m.value === 'organize' && showOrganizeNew}
					<Badge class="ml-auto bg-red-500/15 text-red-700 dark:text-red-300">New</Badge>
				{/if}
			</DropdownMenu.Item>
		{/each}
	</DropdownMenu.Content>
</DropdownMenu.Root>
