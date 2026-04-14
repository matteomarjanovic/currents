<script lang="ts">
	import { BalancedMasonryGrid, Frame } from '@masonry-grid/svelte';
	import type { SaveView } from '$lib/types';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import ImageCard from '$lib/components/image-card.svelte';

	interface Props {
		items: SaveView[];
		loading: boolean;
	}

	let { items, loading }: Props = $props();
</script>

<BalancedMasonryGrid frameWidth={200} gap={16}>
	{#each items as item (item.uri)}
		<Frame width={item.width ?? 3} height={item.height ?? 4}>
			<ImageCard {item} />
		</Frame>
	{/each}
	{#if loading}
		<Frame width={3} height={4}>
			<Skeleton class="h-full w-full rounded-lg" />
		</Frame>
		<Frame width={2} height={3}>
			<Skeleton class="h-full w-full rounded-lg" />
		</Frame>
	{/if}
</BalancedMasonryGrid>
