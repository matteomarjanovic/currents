<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import PersonalizationButton from '$lib/components/personalization-button-v3.svelte';
	import { findFeedLevel } from '$lib/feed-levels';
	import { auth } from '$lib/stores/auth.svelte';

	const level = $derived(findFeedLevel(page.params.level));

	// Unknown slugs and auth-only feeds for logged-out visitors fall back to general.
	$effect(() => {
		if (!level) {
			goto('/explore/general', { replaceState: true });
		} else if (auth.checked && !auth.user && level.value !== 0) {
			goto('/explore/general', { replaceState: true });
		}
	});

	const feed = useInfiniteScroll(async (cursor) => {
		const params = new URLSearchParams({
			personalized: String(level?.value ?? 0),
			limit: '50',
			excludeSaved: 'true'
		});
		if (cursor) params.set('cursor', cursor);

		const res = await fetch(`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getFeed?${params}`, {
			credentials: 'include'
		});
		const data = await res.json();
		return { items: data.feed, cursor: data.cursor };
	});

	let sentinel: HTMLDivElement = $state(undefined!);

	$effect(() => {
		void level?.value;
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

<div class="fixed right-5 bottom-5 z-20">
	<PersonalizationButton />
</div>
