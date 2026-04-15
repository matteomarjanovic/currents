<script lang="ts">
	import type { CollectionView } from '$lib/types';

	let { collection }: { collection: CollectionView } = $props();

	const previews = $derived(collection.previewImages ?? []);
	const cells = $derived([0, 1, 2, 3].map((i) => previews[i]));
</script>

<a href={`/collection/${encodeURIComponent(collection.uri)}`} class="group block">
	<div
		class="grid aspect-square grid-cols-2 grid-rows-2 gap-0.5 overflow-hidden rounded-lg bg-muted"
	>
		{#each cells as src, i (i)}
			{#if src}
				<img {src} alt="" loading="lazy" class="h-full w-full object-cover" />
			{:else}
				<div class="h-full w-full bg-muted"></div>
			{/if}
		{/each}
	</div>
	<div class="mt-2 px-1">
		<div class="truncate text-sm font-medium text-foreground">{collection.name}</div>
		{#if collection.saveCount != null}
			<div class="text-xs text-muted-foreground">
				{collection.saveCount}
				{collection.saveCount === 1 ? 'save' : 'saves'}
			</div>
		{/if}
	</div>
</a>
