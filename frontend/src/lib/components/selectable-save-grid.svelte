<script lang="ts">
	import { BalancedMasonryGrid, Frame } from '@masonry-grid/svelte';
	import { getImageContent, type SaveView } from '$lib/types';
	import { effectiveVisibility, shouldHide } from '$lib/stores/moderation-prefs.svelte';
	import Check from '@lucide/svelte/icons/check';

	interface Props {
		items: SaveView[];
		selected: Set<string>;
		onToggle: (uri: string) => void;
	}

	let { items, selected, onToggle }: Props = $props();

	// Drop hidden-by-prefs saves (same as the main grid). Resaves stay visible but
	// aren't selectable — only originators self-label.
	let visibleItems = $derived(items.filter((i) => !shouldHide(i.labels)));

	let containerWidth = $state<number | undefined>();
	const gap = 16;
	let frameWidth = $derived(
		containerWidth !== undefined && containerWidth < 624
			? Math.max(120, Math.floor((containerWidth - gap - 2) / 2))
			: 200
	);

	function eligible(item: SaveView): boolean {
		return !item.resaveOf && !!getImageContent(item);
	}
</script>

<div bind:clientWidth={containerWidth}>
	<BalancedMasonryGrid {frameWidth} {gap}>
		{#each visibleItems as item (item.uri)}
			{@const image = getImageContent(item)}
			{@const ok = eligible(item)}
			{@const isSelected = selected.has(item.uri)}
			{@const blurred = effectiveVisibility(item.labels) === 'blur'}
			<Frame width={image?.width ?? 3} height={image?.height ?? 4}>
				<button
					type="button"
					disabled={!ok}
					aria-pressed={isSelected}
					onclick={() => ok && onToggle(item.uri)}
					class="group relative block h-full w-full overflow-hidden rounded-lg {ok
						? 'cursor-pointer'
						: 'cursor-not-allowed'}"
				>
					{#if image}
						<img
							src={image.imageUrl}
							alt=""
							loading="lazy"
							class="h-full w-full object-cover {blurred ? 'blur-md' : ''} {ok
								? ''
								: 'opacity-40'}"
						/>
					{:else}
						<div
							class="flex h-full w-full items-center justify-center bg-muted text-xs text-muted-foreground"
						>
							Unsupported
						</div>
					{/if}

					{#if ok}
						<div
							class="absolute inset-0 rounded-lg transition-colors {isSelected
								? 'bg-primary/20 ring-2 ring-primary ring-inset'
								: 'group-hover:bg-black/10'}"
						></div>
						<div
							class="absolute top-2 left-2 flex size-6 items-center justify-center rounded-full border-2 transition-colors {isSelected
								? 'border-primary bg-primary text-primary-foreground'
								: 'border-white/90 bg-black/30'}"
						>
							{#if isSelected}<Check class="size-4" />{/if}
						</div>
					{:else}
						<div
							class="absolute top-2 left-2 rounded-full bg-black/60 px-2 py-0.5 text-[10px] font-medium text-white"
						>
							Resave
						</div>
					{/if}
				</button>
			</Frame>
		{/each}
	</BalancedMasonryGrid>
</div>
