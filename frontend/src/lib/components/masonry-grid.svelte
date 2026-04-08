<script lang="ts">
	import type { SaveView } from '$lib/types';
	import { Skeleton } from '$lib/components/ui/skeleton';

	interface Props {
		items: SaveView[];
		loading: boolean;
	}

	let { items, loading }: Props = $props();
</script>

<div class="columns-2 gap-4 md:columns-4 lg:columns-5 xl:columns-6">
	{#each items as item (item.uri)}
		<div class="mb-4 break-inside-avoid">
			<img
				src={item.imageUrl}
				alt={item.text ?? ''}
				loading="lazy"
				class="w-full rounded-lg"
				style={item.width && item.height
					? `aspect-ratio: ${item.width} / ${item.height}`
					: undefined}
			/>
		</div>
	{/each}

	{#if loading}
		{#each { length: 12 } as _}
			<div class="mb-4 break-inside-avoid">
				<Skeleton class="w-full rounded-lg" style="aspect-ratio: 3 / 4" />
			</div>
		{/each}
	{/if}
</div>
