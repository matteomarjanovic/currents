<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { Toggle } from '$lib/components/ui/toggle';
	import { setCollectionPinned } from '$lib/stores/collections.svelte';
	import { cn } from '$lib/utils';
	import type { CollectionView } from '$lib/types';
	import Pin from '@lucide/svelte/icons/pin';

	let {
		collection,
		placement = 'action',
		class: className
	}: {
		collection: CollectionView;
		placement?: 'action' | 'icon';
		class?: string;
	} = $props();
	let loading = $state(false);
	let pinned = $derived(!!collection.viewer?.pinned);

	async function setPinned(next: boolean) {
		if (loading || next === pinned) return;
		loading = true;
		if (!(await setCollectionPinned(collection.uri, next))) {
			toast.error(`Couldn't ${next ? 'pin' : 'unpin'} '${collection.name}'`);
		}
		loading = false;
	}
</script>

<Toggle
	pressed={pinned}
	onPressedChange={setPinned}
	disabled={loading}
	aria-label={pinned ? `Unpin ${collection.name}` : `Pin ${collection.name}`}
	title={pinned ? 'Unpin' : 'Pin'}
	data-sidebar={placement === 'action' ? 'menu-action' : undefined}
	size="sm"
	class={cn(
		'pointer-events-none absolute z-10 size-6 min-w-0 bg-transparent p-0 opacity-0 hover:bg-transparent aria-pressed:bg-transparent data-[state=on]:pointer-events-auto data-[state=on]:opacity-100',
		placement === 'icon'
			? 'top-1 left-2 group-focus-within/menu-item:pointer-events-auto group-focus-within/menu-item:opacity-100 group-hover/menu-item:pointer-events-auto group-hover/menu-item:opacity-100'
			: 'top-0.5 right-1 group-focus-within/menu-sub-item:pointer-events-auto group-focus-within/menu-sub-item:opacity-100 group-hover/menu-sub-item:pointer-events-auto group-hover/menu-sub-item:opacity-100',
		className
	)}
>
	<Pin class="size-3.5 group-hover/toggle:fill-current {pinned ? 'fill-current' : ''}" />
</Toggle>
