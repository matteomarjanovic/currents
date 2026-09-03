<script lang="ts">
	import { BalancedMasonryGrid, Frame } from '@masonry-grid/svelte';
	import { getImageContent, type SaveView } from '$lib/types';
	import { tileRatio } from '$lib/image-ratio';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import ImageCard from '$lib/components/image-card.svelte';
	import { setSaveSequence, syncSaveSequence } from '$lib/save-sequence.svelte';
	import { shouldHide } from '$lib/stores/moderation-prefs.svelte';
	import { isHiddenFeedImage } from '$lib/stores/hidden-feed-images.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { queueExploreSaveSuggestions } from '$lib/stores/save-suggestions.svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { onDestroy, untrack } from 'svelte';

	interface Props {
		items: SaveView[];
		loading: boolean;
		// Forwarded to each ImageCard; false makes tiles non-clickable.
		linkToDetail?: boolean;
		// Forwarded to each ImageCard; shows an always-visible Save button on mobile.
		mobileSave?: boolean;
		// Forwarded to each ImageCard; long-pressing a tile opens Quick actions.
		longPressSave?: boolean;
		// The Explore feed opts into the viewer's newly hidden image set so a
		// successful action removes the tile immediately without affecting other grids.
		discoveryFeed?: boolean;
		// This grid's own "load the next page". Passing it lets the detail view keep
		// swiping past what was on screen when the tile was tapped: it asks for another
		// page as the viewer nears the end of the run, since this grid's scroll sentinel
		// is under the overlay and never fires while the detail is open.
		loadMore?: () => void | Promise<void>;
	}

	let {
		items,
		loading,
		linkToDetail = true,
		mobileSave = false,
		longPressSave = false,
		discoveryFeed = false,
		loadMore
	}: Props = $props();

	// Drop saves the viewer has set to "hide" before the grid sees them: no
	// card is rendered, no Frame reserved, and the <img> is never fetched.
	let visibleItems = $derived(
		items.filter((i) => !shouldHide(i.labels) && (!discoveryFeed || !isHiddenFeedImage(i)))
	);

	// Identity for this grid instance, so the store can tell whether the run it holds is
	// still ours to extend.
	const gridId = {};
	// Preserve server-rendered/initial cards. Pages appended after mount get the
	// viewport-driven treatment below; initial SSR markup must hydrate unchanged.
	const initiallyRendered = new Set(untrack(() => items.map((item) => item.uri)));
	// Frames are cheap and give the masonry its complete geometry immediately. The
	// interactive card subtree is not: defer it until the frame is near enough to
	// be seen, then keep it mounted so card state survives scrolling away and back.
	const nearby = new SvelteSet<string>();
	const observedUris = new WeakMap<Element, string>();
	let cardObserver: IntersectionObserver | undefined;

	function renderNearViewport(node: HTMLElement, uri: string) {
		if (!('IntersectionObserver' in window)) {
			nearby.add(uri);
			return;
		}
		observedUris.set(node, uri);
		cardObserver ??= new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (!entry.isIntersecting) continue;
					const itemUri = observedUris.get(entry.target);
					if (itemUri) nearby.add(itemUri);
					cardObserver?.unobserve(entry.target);
				}
			},
			{ rootMargin: '800px 0px' }
		);
		cardObserver.observe(node);
		return {
			destroy() {
				cardObserver?.unobserve(node);
			}
		};
	}

	onDestroy(() => cardObserver?.disconnect());

	// Feed newly loaded items into the run while we own it. Only grids that can load
	// more take part: the detail's related rail refills with a different image's saves
	// on every swipe, and that must never leak into the run being swiped through.
	$effect(() => {
		if (loadMore) syncSaveSequence(gridId, visibleItems);
		if (auth.user) queueExploreSaveSuggestions(visibleItems.map((item) => item.uri));
	});

	let containerWidth = $state<number | undefined>();
	let viewportWidth = $state<number | undefined>();
	let viewportHeight = $state<number | undefined>();
	const gap = 16;
	const minFrameWidth = 200;
	const fullHDWidth = 1920;
	let maxColumns = $derived(viewportWidth !== undefined && viewportWidth > fullHDWidth ? 8 : 7);
	let frameWidth = $derived(
		containerWidth !== undefined && containerWidth < 624
			? Math.max(120, Math.floor((containerWidth - gap - 2) / 2))
			: containerWidth !== undefined
				? Math.max(
						minFrameWidth,
						Math.floor((containerWidth - (maxColumns - 1) * gap) / maxColumns)
					)
				: minFrameWidth
	);

	const skeletonShapes: Array<[number, number]> = [
		[3, 4],
		[2, 3],
		[4, 5],
		[1, 1],
		[3, 5],
		[4, 3],
		[3, 4],
		[5, 7],
		[2, 3],
		[4, 5],
		[1, 1],
		[3, 4],
		[3, 5],
		[4, 3],
		[2, 3],
		[3, 4],
		[4, 5],
		[3, 4]
	];

	let skeletonCount = $derived.by(() => {
		if (containerWidth === undefined || viewportHeight === undefined) return 8;
		const cols = Math.max(1, Math.floor((containerWidth + gap) / (frameWidth + gap)));
		const avgFrameHeight = frameWidth * (4 / 3);
		const rows = Math.ceil(viewportHeight / (avgFrameHeight + gap)) + 1;
		return Math.min(skeletonShapes.length, cols * rows);
	});
</script>

<svelte:window bind:innerWidth={viewportWidth} bind:innerHeight={viewportHeight} />

<div bind:clientWidth={containerWidth}>
	<!-- Appended frames briefly have the default CSS order before the masonry
	     observer positions them. Keep that transient reflow from becoming the
	     browser's scroll anchor and moving the viewport as a page lands. -->
	<BalancedMasonryGrid {frameWidth} {gap} style="overflow-anchor: none;">
		{#each visibleItems as item, i (item.uri)}
			{@const image = getImageContent(item)}
			{@const ratio = tileRatio(image?.width, image?.height)}
			<!-- The frame owns its size, so off-screen contents can be skipped without
			     changing masonry geometry or the scroll position. -->
			<Frame width={ratio.width} height={ratio.height} style="content-visibility: auto;">
				<div
					class="h-full w-full overflow-hidden rounded-lg"
					style={image?.dominantColor ? `background-color: ${image.dominantColor}` : undefined}
					use:renderNearViewport={item.uri}
				>
					{#if initiallyRendered.has(item.uri) || i < skeletonCount || nearby.has(item.uri)}
						<!-- Hand the detail view the images either side of this one, so it can be
						     swiped through on touch. Same list the user is looking at, minus what
						     their moderation prefs hide. -->
						<ImageCard
							{item}
							{linkToDetail}
							{mobileSave}
							{longPressSave}
							preloadControls={nearby.has(item.uri)}
							onOpen={() => setSaveSequence(gridId, visibleItems, loadMore)}
						/>
					{/if}
				</div>
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
