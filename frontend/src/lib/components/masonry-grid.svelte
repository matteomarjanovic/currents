<script lang="ts">
	import { BalancedMasonryGrid, Frame } from '@masonry-grid/svelte';
	import { getImageContent, type SaveView } from '$lib/types';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import ImageCard from '$lib/components/image-card.svelte';

	interface Props {
		items: SaveView[];
		loading: boolean;
	}

	let { items, loading }: Props = $props();

	let containerWidth = $state<number | undefined>();
	const gap = 16;
	let frameWidth = $derived(
		containerWidth !== undefined && containerWidth < 624
			? Math.max(120, Math.floor((containerWidth - gap - 2) / 2))
			: 200
	);
</script>

<div bind:clientWidth={containerWidth}>
	<BalancedMasonryGrid {frameWidth} {gap}>
	{#each items as item (item.uri)}
		<Frame width={getImageContent(item)?.width ?? 3} height={getImageContent(item)?.height ?? 4}>
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
</div>
