<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import { page } from '$app/state';
	import { apiFetch } from '$lib/api';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import PullToRefresh from '$lib/components/pull-to-refresh.svelte';
	import PersonalizationButton from '$lib/components/personalization-button-v3.svelte';
	import { Button } from '$lib/components/ui/button';
	import Compass from '@lucide/svelte/icons/compass';
	import Upload from '@lucide/svelte/icons/upload';
	import Download from '@lucide/svelte/icons/download';
	import { findFeedLevel, type FeedLevelSlug } from '$lib/feed-levels';
	import {
		exploreFeedScope,
		resumeExploreFeed,
		rememberExploreFeed,
		forgetExploreFeed,
		type FeedPosition
	} from '$lib/explore-feed';
	import { bodyScrollY, scrollBodyTo } from '$lib/scroll-lock';
	import {
		feedPreferences,
		feedPreferencesLoaded,
		loadFeedPreferences
	} from '$lib/stores/feed-preferences.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { preferences } from '$lib/stores/preferences.svelte';
	import {
		exploreFeedRestoration,
		finishExploreFeedReturn
	} from '$lib/stores/explore-feed-restoration.svelte';
	import {
		endExploreSaveSession,
		startExploreSaveSession
	} from '$lib/stores/save-suggestions.svelte';

	const level = $derived(findFeedLevel(page.params.level));
	const scope = $derived(
		exploreFeedScope(auth.user?.did ?? '', feedPreferences.excludedCollections)
	);
	let activeLevel: FeedLevelSlug | undefined;
	let activeScope = '';
	let positionBeforeOverlay: FeedPosition | undefined;
	let ready = $state(false);
	let pendingPosition = $state<FeedPosition | null>(null);
	let gridRoot: HTMLDivElement | undefined = $state();

	onMount(() => {
		startExploreSaveSession(preferences.saveSuggestionMode);
		return () => {
			endExploreSaveSession();
			exploreFeedRestoration.holdingOverlay = false;
		};
	});
	$effect(() => {
		if (auth.checked && auth.user) {
			untrack(() => {
				if (!feedPreferencesLoaded.value) void loadFeedPreferences();
			});
		}
	});

	// Unknown slugs and auth-only feeds for logged-out visitors fall back to general.
	$effect(() => {
		if (!level) {
			goto('/explore/general', { replace: true });
		} else if (auth.checked && !auth.user && level.value !== 0) {
			goto('/explore/general', { replace: true });
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
		if (!res.ok) throw new Error(data.error === 'ExpiredCursor' ? 'ExpiredCursor' : 'FeedError');
		return { items: data.feed, cursor: data.cursor };
	});

	function rememberFeed() {
		if (!activeLevel || pendingPosition || (feed.items.length === 0 && feed.hasMore)) return;
		const visible = [...(gridRoot?.querySelectorAll<HTMLElement>('[data-save-uri]') ?? [])]
			.map((element) => ({ uri: element.dataset.saveUri!, rect: element.getBoundingClientRect() }))
			.filter(({ rect }) => rect.bottom > 0 && rect.top < window.innerHeight)
			.sort((a, b) => a.rect.top - b.rect.top || a.rect.left - b.rect.left)[0];
		const position =
			document.body.classList.contains('scroll-locked') && positionBeforeOverlay
				? positionBeforeOverlay
				: {
						y: bodyScrollY(),
						anchor: visible ? { uri: visible.uri, top: visible.rect.top } : undefined
					};
		positionBeforeOverlay = position;
		rememberExploreFeed(activeScope, activeLevel, {
			items: $state.snapshot(feed.items),
			cursor: feed.cursor,
			error: feed.error,
			position
		});
	}

	beforeNavigate((navigation) => {
		if (!navigation.shallow) rememberFeed();
	});

	let overlayWasOpen = false;
	$effect.pre(() => {
		const overlayOpen = !!page.state.save || (page.state.saveStack?.length ?? 0) > 0;
		if (overlayWasOpen && !overlayOpen) {
			// Body unlocking restores synchronously, but browser/history restoration
			// can run later. Reapply the feed anchor after the normal grid settles.
			untrack(() => {
				if (!pendingPosition && positionBeforeOverlay) pendingPosition = positionBeforeOverlay;
				if (pendingPosition) exploreFeedRestoration.holdingOverlay = true;
			});
		}
		overlayWasOpen = overlayOpen;
	});

	// Restore pages before scrolling, including when SvelteKit reuses this route
	// for a different feed. An old request cannot append to the newly active feed.
	$effect(() => {
		if (!auth.checked || !level || (!auth.user && level.value !== 0)) return;
		const slug = level.slug;
		const nextScope = scope;
		untrack(() => {
			if (activeLevel === slug && activeScope === nextScope) return;
			if (activeScope === nextScope) rememberFeed();
			activeLevel = slug;
			activeScope = nextScope;
			const cached = resumeExploreFeed(nextScope, slug);
			positionBeforeOverlay = cached?.position;
			feed.reset(cached);
			pendingPosition = cached && cached.items.length > 0 ? cached.position : null;
			if (!pendingPosition) finishExploreFeedReturn();
			ready = true;
			if (!cached) {
				scrollBodyTo(0);
				void feed.loadMore();
			}
		});
	});

	// Restore before the next paint, then refine the anchor on each frame until
	// masonry settles. Waiting to scroll until it settles exposes the page's top.
	$effect(() => {
		if (!gridRoot || !pendingPosition) return;
		const root = gridRoot;
		const position = pendingPosition;
		// The locked body's width can differ from the normal page (scrollbar
		// gutter). Keep the saved offset underneath an image; align its anchor
		// only after that overlay closes and the normal grid width is restored.
		if (page.state.save || (page.state.saveStack?.length ?? 0) > 0) {
			scrollBodyTo(position.y);
			return;
		}
		scrollBodyTo(position.y);
		let previousHeight = -1;
		let frame: number;
		const restore = () => {
			const height = root.getBoundingClientRect().height;
			const anchor = position.anchor
				? [...root.querySelectorAll<HTMLElement>('[data-save-uri]')].find(
						(element) => element.dataset.saveUri === position.anchor?.uri
					)
				: undefined;
			const y = anchor
				? bodyScrollY() + anchor.getBoundingClientRect().top - position.anchor!.top
				: position.y;
			scrollBodyTo(Math.max(0, y));
			if (height === previousHeight) {
				pendingPosition = null;
				exploreFeedRestoration.holdingOverlay = false;
				finishExploreFeedReturn();
			} else {
				previousHeight = height;
				frame = requestAnimationFrame(restore);
			}
		};
		frame = requestAnimationFrame(restore);
		return () => cancelAnimationFrame(frame);
	});

	// Personal / New worlds rank against the user's collections; a brand-new user
	// gets nothing back. Show them how to get started instead of a blank grid.
	const showEmpty = $derived(
		!!level && level.value !== 0 && !feed.loading && !feed.hasMore && feed.items.length === 0
	);

	let sentinel: HTMLDivElement = $state(undefined!);
	const feedLoadAhead = 1200;

	function loadFeedIfNearEnd() {
		if (
			!ready ||
			pendingPosition ||
			!sentinel ||
			!feed.hasMore ||
			feed.loading ||
			sentinel.getBoundingClientRect().top > window.innerHeight + feedLoadAhead
		)
			return;
		void feed.loadMore();
	}

	function refreshFeed() {
		if (activeLevel) forgetExploreFeed(activeScope, activeLevel);
		pendingPosition = null;
		scrollBodyTo(0);
		feed.reset();
		return feed.loadMore();
	}

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
				if (entries[0].isIntersecting) loadFeedIfNearEnd();
			},
			{ rootMargin: `${feedLoadAhead}px 0px` }
		);
		observer.observe(sentinel);

		// IntersectionObserver can miss a target whose position changes during the
		// masonry reflow. Keep the same early-load threshold as a cheap fallback on
		// scroll/resize, and check once more after Svelte and masonry have painted.
		const check = () => loadFeedIfNearEnd();
		window.addEventListener('scroll', check, { passive: true });
		window.addEventListener('resize', check);
		const frame = requestAnimationFrame(check);
		return () => {
			observer.disconnect();
			window.removeEventListener('scroll', check);
			window.removeEventListener('resize', check);
			cancelAnimationFrame(frame);
		};
	});

	// A page landing changes the sentinel's position without necessarily changing
	// its intersection state. Re-check after each append so the next request is not
	// dependent on a later viewport resize.
	$effect(() => {
		void feed.items.length;
		void ready;
		void pendingPosition;
		if (!sentinel) return;
		const frame = requestAnimationFrame(loadFeedIfNearEnd);
		return () => cancelAnimationFrame(frame);
	});
</script>

<PullToRefresh label="feed" disabled={feed.loading} onRefresh={refreshFeed} />

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
	<div bind:this={gridRoot}>
		<MasonryGrid
			items={feed.items}
			loading={feed.loading}
			loadMore={feed.loadMore}
			onOpen={rememberFeed}
			longPressSave
			discoveryFeed
		/>
	</div>
	{#if feed.error}
		<div role="alert" class="flex flex-col items-center gap-3 py-8 text-center">
			<p class="text-sm text-muted-foreground">
				{feed.error instanceof Error && feed.error.message === 'ExpiredCursor'
					? 'The feed has refreshed. Load the latest images to keep exploring.'
					: 'Couldn’t load the feed. Refresh to try again.'}
			</p>
			<Button variant="outline" onclick={refreshFeed}>Refresh feed</Button>
		</div>
	{:else if feed.hasMore}
		<div bind:this={sentinel} class="h-1"></div>
	{/if}
{/if}

<!-- right = padding + scrollbar width, anchored to the window edge: the
     `100% - 100vw` term cancels out viewport-width changes when a scroll lock
     hides the scrollbar, so the button never moves; the latched --sb places it
     on the grid's border while the scrollbar is showing. -->
<div
	class="fixed right-[calc(0.5rem+var(--sb)+100%-100vw)] bottom-[calc(env(safe-area-inset-bottom)-0.125rem)] z-20 hidden md:right-[calc(1rem+var(--sb)+100%-100vw)] md:bottom-[calc(env(safe-area-inset-bottom)+0.875rem)] md:flex"
	style="--sb: {scrollbarInset}px"
>
	<PersonalizationButton />
</div>
