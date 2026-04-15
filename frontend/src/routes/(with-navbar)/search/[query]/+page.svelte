<script lang="ts">
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { personalization } from '$lib/stores/personalization.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';

	const search = useInfiniteScroll(async (cursor) => {
		const q = page.params.query ?? '';
		const params = new URLSearchParams({
			q,
			personalized: String(personalization.value),
			limit: '50',
			excludeSaved: 'true'
		});
		if (cursor) params.set('cursor', cursor);

		const res = await fetch(
			`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.searchSaves?${params}`,
			{ credentials: 'include' }
		);
		const data = await res.json();
		return { items: data.saves, cursor: data.cursor };
	});

	let sentinel: HTMLDivElement = $state(undefined!);

	$effect(() => {
		void page.params.query;
		void personalization.value;
		const timeout = setTimeout(() => {
			untrack(() => {
				search.reset();
				search.loadMore();
			});
		}, 300);
		return () => clearTimeout(timeout);
	});

	$effect(() => {
		if (!sentinel) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) search.loadMore();
			},
			{ rootMargin: '400px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});
</script>

<MasonryGrid items={search.items} loading={search.loading} />

{#if search.hasMore}
	<div bind:this={sentinel} class="h-1"></div>
{/if}
