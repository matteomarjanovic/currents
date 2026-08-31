<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Drawer from '$lib/components/ui/drawer';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import ImageActionGrid from '$lib/components/image-action-grid.svelte';
	import { drawerScrollSwipe } from '$lib/drawer-scroll-swipe';
	import type { SaveView } from '$lib/types';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';

	let {
		item,
		open = $bindable(false),
		onSavesChange
	}: {
		item: SaveView;
		open?: boolean;
		onSavesChange?: (saves: { collectionUri: string; saveUri: string }[]) => void;
	} = $props();

	let view = $state<'actions' | 'collections'>('actions');

	$effect(() => {
		if (!open) view = 'actions';
	});

	function handleSavesChange(saves: { collectionUri: string; saveUri: string }[]) {
		onSavesChange?.(saves);
		open = false;
	}
</script>

<Drawer.Root bind:open>
	<Drawer.Content>
		{#if view === 'actions'}
			<Drawer.Header>
				<Drawer.Title>Quick actions</Drawer.Title>
				<Drawer.Description>Save, copy, or share this image.</Drawer.Description>
			</Drawer.Header>
			<div class="px-4">
				<ImageActionGrid {item} onAction={() => (open = false)} />
			</div>
			<div
				class="mt-4 flex flex-col gap-2 border-t border-border px-4 pt-4"
				style="padding-bottom: calc(env(safe-area-inset-bottom) + 1rem)"
			>
				<CollectionSelector {item} variant="quick" onSavesChange={handleSavesChange} />
				<Button class="w-full" onclick={() => (view = 'collections')}>Save somewhere else</Button>
			</div>
		{:else}
			<Drawer.Header class="relative">
				<Button
					variant="ghost"
					size="icon-sm"
					class="absolute top-3 left-4"
					onclick={() => (view = 'actions')}
					aria-label="Back to quick actions"
				>
					<ChevronLeft />
				</Button>
				<Drawer.Title>Save somewhere else</Drawer.Title>
				<Drawer.Description>Choose a collection, section, or your profile.</Drawer.Description>
			</Drawer.Header>
			<div
				class="max-h-[60vh] touch-auto overflow-y-auto overscroll-contain px-4"
				style="padding-bottom: calc(env(safe-area-inset-bottom) + 1rem)"
				use:drawerScrollSwipe={() => (open = false)}
			>
				<CollectionSelector {item} variant="inline" onSavesChange={handleSavesChange} />
			</div>
		{/if}
	</Drawer.Content>
</Drawer.Root>
