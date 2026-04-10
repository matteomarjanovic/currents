<script lang="ts">
	import type { SaveView } from '$lib/types';
	import { Skeleton } from '$lib/components/ui/skeleton';

	interface Props {
		items: SaveView[];
		loading: boolean;
	}

	let { items, loading }: Props = $props();

	function getColumnCount(width: number): number {
		if (width >= 1280) return 6;
		if (width >= 1024) return 5;
		if (width >= 768) return 4;
		return 2;
	}

	let windowWidth = $state(typeof window !== 'undefined' ? window.innerWidth : 1024);

	$effect(() => {
		function onResize() {
			windowWidth = window.innerWidth;
		}
		window.addEventListener('resize', onResize);
		return () => window.removeEventListener('resize', onResize);
	});

	let columnCount = $derived(getColumnCount(windowWidth));

	let columns = $derived.by(() => {
		const cols: SaveView[][] = Array.from({ length: columnCount }, () => []);
		const heights = new Array(columnCount).fill(0);

		for (const item of items) {
			const shortest = heights.indexOf(Math.min(...heights));
			cols[shortest].push(item);
			heights[shortest] += item.width && item.height ? item.height / item.width : 4 / 3;
		}

		return cols;
	});
</script>

<div class="flex gap-4">
	{#each columns as column, i (i)}
		<div class="flex flex-1 flex-col gap-4">
			{#each column as item (item.uri)}
				<img
					src={item.imageUrl}
					alt={item.text ?? ''}
					loading="lazy"
					class="w-full rounded-lg"
					style={item.width && item.height
						? `aspect-ratio: ${item.width} / ${item.height}`
						: undefined}
				/>
			{/each}
			{#if loading}
				<Skeleton class="w-full rounded-lg" style="aspect-ratio: 3 / 4" />
				<Skeleton class="w-full rounded-lg" style="aspect-ratio: 2 / 3" />
			{/if}
		</div>
	{/each}
</div>
