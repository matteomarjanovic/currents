<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { apiFetch } from '$lib/api';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import PersonalizationButton from '$lib/components/personalization-button-v3.svelte';
	import { Button } from '$lib/components/ui/button';
	import Compass from '@lucide/svelte/icons/compass';
	import Upload from '@lucide/svelte/icons/upload';
	import Download from '@lucide/svelte/icons/download';
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

		const res = await apiFetch(`/xrpc/is.currents.feed.getFeed?${params}`);
		const data = await res.json();
		return { items: data.feed, cursor: data.cursor };
	});

	// Personal / New worlds rank against the user's collections; a brand-new user
	// gets nothing back. Show them how to get started instead of a blank grid.
	const showEmpty = $derived(
		!!level && level.value !== 0 && !feed.loading && !feed.hasMore && feed.items.length === 0
	);

	let sentinel: HTMLDivElement = $state(undefined!);

	// Align the flow-field button with the grid's right border: content padding
	// plus the root scrollbar's width. Scroll locks (menus, dialogs, the save
	// overlay) report 0 while they temporarily hide the scrollbar, so the width
	// only ratchets up — the button must not shift when a menu opens.
	let scrollbarInset = $state(0);
	function measureInset() {
		const w = window.innerWidth - document.documentElement.clientWidth;
		if (w > 0) scrollbarInset = w;
	}
	onMount(() => {
		window.addEventListener('resize', measureInset);
		return () => window.removeEventListener('resize', measureInset);
	});
	// Re-measure as feed pages land: the scrollbar may only appear once the grid
	// outgrows the viewport.
	$effect(() => {
		void feed.items.length;
		measureInset();
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

	// The sentinel's observer above covers the initial load; this effect only
	// needs to react to the level actually *changing* (switching feed levels
	// reuses this component — the route param changes, not the instance). It
	// still runs on mount too, since `$effect` always fires once immediately —
	// skip that first run, or every page load would reset and refetch a feed
	// that just loaded, tearing down tiles (and anything mid-interaction with
	// one, like a long press) out from under themselves.
	let skipNextLevelEffect = true;
	$effect(() => {
		void level?.value;
		if (skipNextLevelEffect) {
			skipNextLevelEffect = false;
			return;
		}
		const timeout = setTimeout(() => {
			untrack(() => {
				feed.reset();
				feed.loadMore();
			});
		}, 300);
		return () => clearTimeout(timeout);
	});
</script>

<svelte:head>
	<title>Explore · Currents</title>
</svelte:head>

{#if showEmpty}
	<div
		class="mx-auto flex min-h-[60vh] max-w-md flex-col items-center justify-center gap-6 px-4 text-center"
	>
		<div>
			<h2 class="text-lg font-medium text-foreground">Start personalising your feed!</h2>
			<p class="mt-2 text-sm text-muted-foreground">
				Your Personal and New worlds feeds are built from the images you save to collections. Save a
				few to start seeing picks tailored to your taste.
			</p>
		</div>
		<div class="flex flex-col items-center justify-center gap-2">
			<Button href="/explore/general">
				<Compass class="size-4" />
				Explore the general feed
			</Button>
			<Button href="/upload" variant="outline">
				<Upload class="size-4" />
				Upload an image
			</Button>
			<Button href="/import/pinterest" variant="outline">
				<Download class="size-4" />
				Import from Pinterest
			</Button>
		</div>
	</div>
{:else}
	<MasonryGrid items={feed.items} loading={feed.loading} longPressSave />
	{#if feed.hasMore}
		<div bind:this={sentinel} class="h-1"></div>
	{/if}
{/if}

<!-- right = padding + scrollbar width, anchored to the window edge: the
     `100% - 100vw` term cancels out viewport-width changes when a scroll lock
     hides the scrollbar, so the button never moves; the latched --sb places it
     on the grid's border while the scrollbar is showing. -->
<div
	class="fixed right-[calc(0.5rem+var(--sb)+100%-100vw)] bottom-[calc(env(safe-area-inset-bottom)+1.25rem)] z-20 flex md:right-[calc(1rem+var(--sb)+100%-100vw)]"
	style="--sb: {scrollbarInset}px"
>
	<PersonalizationButton />
</div>
