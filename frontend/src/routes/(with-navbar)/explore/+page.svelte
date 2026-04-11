<script lang="ts">
	import { untrack } from 'svelte';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { personalization } from '$lib/stores/personalization.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';

	const feed = useInfiniteScroll(async (cursor) => {
		const params = new URLSearchParams({
			personalized: String(personalization.value),
			limit: '50'
		});
		if (cursor) params.set('cursor', cursor);

		const res = await fetch(
			`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getFeed?${params}`,
			{ credentials: 'include' }
		);
		const data = await res.json();
		return { items: data.feed, cursor: data.cursor };
	});

	let sentinel: HTMLDivElement = $state(undefined!);

	$effect(() => {
		void personalization.value;
		const timeout = setTimeout(() => {
			untrack(() => {
				feed.reset();
				feed.loadMore();
			});
		}, 300);
		return () => clearTimeout(timeout);
	});

	$effect(() => {
		if (!sentinel) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) feed.loadMore();
			},
			{ rootMargin: '400px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});
</script>

<MasonryGrid items={feed.items} loading={feed.loading} />
{#if feed.hasMore}
	<div bind:this={sentinel} class="h-1"></div>
{/if}
