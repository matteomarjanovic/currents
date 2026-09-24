<script lang="ts">
	import { untrack } from 'svelte';
	import { apiFetch } from '$lib/api';
	import type { SaveView } from '$lib/types';
	import { shouldHide } from '$lib/stores/moderation-prefs.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import { Button } from '$lib/components/ui/button';
	import ImageOff from '@lucide/svelte/icons/image-off';

	let { save, onViewFullScreen }: { save: SaveView; onViewFullScreen: (save: SaveView) => void } =
		$props();

	// Same source as explore mode's "Related" section: pgvector ANN over the whole
	// Currents corpus, seeded by the focused save's visual identity.
	const related = useInfiniteScroll<SaveView>(async (cursor) => {
		const params = new URLSearchParams({ uri: save.uri, limit: '50' });
		if (cursor) params.set('cursor', cursor);
		const res = await apiFetch(`/xrpc/is.currents.feed.getRelatedSaves?${params}`);
		if (!res.ok) throw new Error(`request failed: ${res.status}`);
		const data = await res.json();
		return { items: data.saves ?? [], cursor: data.cursor };
	});

	// Reseed whenever the focused image changes.
	$effect(() => {
		void save.uri;
		untrack(() => {
			related.reset();
			related.loadMore();
			if (scrollEl) scrollEl.scrollTop = 0;
		});
	});

	let visible = $derived(related.items.filter((i) => !shouldHide(i.labels)));

	let scrollEl = $state<HTMLDivElement>();
	let sentinel = $state<HTMLDivElement>();
	$effect(() => {
		if (!sentinel || !scrollEl) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) related.loadMore();
			},
			{ root: scrollEl, rootMargin: '1200px 0px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});
</script>

<div bind:this={scrollEl} class="h-full overflow-y-auto p-3 [overflow-anchor:none]">
	{#if related.error && visible.length === 0}
		<div
			class="flex h-full flex-col items-center justify-center gap-2 text-center text-sm text-muted-foreground"
		>
			<ImageOff class="size-6" />
			<p>Couldn't load related images.</p>
			<Button variant="outline" size="sm" class="mt-1" onclick={() => related.retry()}>
				Try again
			</Button>
		</div>
	{:else if visible.length === 0 && !related.loading}
		<div
			class="flex h-full flex-col items-center justify-center gap-2 text-center text-sm text-muted-foreground"
		>
			<ImageOff class="size-6" />
			<p>No similar images found.</p>
		</div>
	{:else}
		<!-- Desktop cards show Save and image actions on hover. On mobile, tap opens
		     the image viewer and long-press opens the shared Quick actions drawer. -->
		<MasonryGrid
			items={related.items}
			loading={related.loading}
			linkToDetail={false}
			longPressSave
			{onViewFullScreen}
			reservePageScrollbarGutter={false}
		/>
		{#if related.hasMore}
			<div bind:this={sentinel} class="h-1"></div>
		{/if}
	{/if}
</div>
