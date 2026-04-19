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
	let viewportHeight = $state<number | undefined>();
	const gap = 16;
	let frameWidth = $derived(
		containerWidth !== undefined && containerWidth < 624
			? Math.max(120, Math.floor((containerWidth - gap - 2) / 2))
			: 200
	);

	const skeletonShapes: Array<[number, number]> = [
		[3, 4], [2, 3], [4, 5], [1, 1], [3, 5], [4, 3],
		[3, 4], [5, 7], [2, 3], [4, 5], [1, 1], [3, 4],
		[3, 5], [4, 3], [2, 3], [3, 4], [4, 5], [3, 4]
	];

	let skeletonCount = $derived.by(() => {
		if (containerWidth === undefined || viewportHeight === undefined) return 8;
		const cols = Math.max(1, Math.floor((containerWidth + gap) / (frameWidth + gap)));
		const avgFrameHeight = frameWidth * (4 / 3);
		const rows = Math.ceil(viewportHeight / (avgFrameHeight + gap)) + 1;
		return Math.min(skeletonShapes.length, cols * rows);
	});
</script>

<svelte:window bind:innerHeight={viewportHeight} />

<div bind:clientWidth={containerWidth}>
	<BalancedMasonryGrid {frameWidth} {gap}>
	{#each items as item (item.uri)}
		<Frame width={getImageContent(item)?.width ?? 3} height={getImageContent(item)?.height ?? 4}>
			<ImageCard {item} />
		</Frame>
	{/each}
	{#if loading && items.length === 0}
		{#each skeletonShapes.slice(0, skeletonCount) as [w, h], i (i)}
			<Frame width={w} height={h}>
				<Skeleton class="h-full w-full rounded-lg" />
			</Frame>
		{/each}
	{:else if loading}
		<Frame width={3} height={4}>
			<Skeleton class="h-full w-full rounded-lg" />
		</Frame>
		<Frame width={2} height={3}>
			<Skeleton class="h-full w-full rounded-lg" />
		</Frame>
	{/if}
	</BalancedMasonryGrid>
</div>
