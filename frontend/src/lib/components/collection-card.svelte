<script lang="ts">
	import type { CollectionView } from '$lib/types';

	let { collection, sectionCount = 0 }: { collection: CollectionView; sectionCount?: number } =
		$props();

	const previews = $derived(collection.previewImages ?? []);
	const cells = $derived([0, 1, 2, 3].map((i) => previews[i]));
	const href = $derived.by(() => {
		const rkey = collection.uri.split('/').pop() ?? '';
		const handle = collection.author?.handle ?? collection.uri.split('/')[2];
		return `/profile/${handle}/collection/${rkey}`;
	});
</script>

<a {href} class="group block">
	<div
		class="relative grid aspect-square grid-cols-2 grid-rows-2 gap-0.5 overflow-hidden rounded-lg bg-muted"
	>
		{#each cells as src, i (i)}
			{#if src}
				<img {src} alt="" loading="lazy" class="h-full w-full object-cover" />
			{:else}
				<div class="h-full w-full bg-muted"></div>
			{/if}
		{/each}
		{#if sectionCount > 0}
			<div
				class="absolute top-1.5 right-1.5 rounded-full bg-black/60 px-2 py-0.5 text-xs font-medium text-white"
			>
				{sectionCount}
				{sectionCount === 1 ? 'section' : 'sections'}
			</div>
		{/if}
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
