<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { BalancedMasonryGrid, Frame } from '@masonry-grid/svelte';
	import { apiFetch } from '$lib/api';
	import { getImageContent, type SaveView } from '$lib/types';
	import { collections } from '$lib/stores/collections.svelte';
	import { favouriteCollections } from '$lib/stores/favourites.svelte';
	import { requireSupporter } from '$lib/stores/supporter.svelte';
	import { shouldHide } from '$lib/stores/moderation-prefs.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import * as Popover from '$lib/components/ui/popover';
	import * as Command from '$lib/components/ui/command';
	import { Input } from '$lib/components/ui/input';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import LabeledMedia from '$lib/components/labeled-media.svelte';
	import CollectionFilterItems from '$lib/components/organize/collection-filter-items.svelte';
	import SidebarUserMenu from '$lib/components/sidebar-user-menu.svelte';
	import Logo from '$lib/assets/logo.svelte';
	import ModeSwitcher from '$lib/components/mode-switcher.svelte';
	import SearchIcon from '@lucide/svelte/icons/search';
	import ListFilter from '@lucide/svelte/icons/list-filter';
	import ImageOff from '@lucide/svelte/icons/image-off';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	import X from '@lucide/svelte/icons/x';

	// The input text is a draft; `searchQuery` (set on Enter) is the semantic
	// query driving the grid. The collection scope filters both browsing and
	// searching. Clearing the input restores the browse view.
	let input = $state('');
	let searchQuery = $state<string | null>(null);
	const scope = new SvelteSet<string>();
	let scopeUris = $derived([...scope]);
	$effect(() => {
		if (!input.trim()) searchQuery = null;
	});

	function toggleScope(uri: string) {
		if (scope.has(uri)) scope.delete(uri);
		else scope.add(uri);
	}

	// Library search is supporter-tier (the server enforces it too with a 403).
	// Gate once per typed query on the first character — the paywall opens
	// early instead of on a doomed submit — and re-check on submit in case
	// typing raced the async gate.
	let gateChecked = false;
	$effect(() => {
		const has = !!input.trim();
		untrack(() => {
			if (!has) {
				gateChecked = false;
				return;
			}
			if (gateChecked) return;
			gateChecked = true;
			void requireSupporter().then((ok) => {
				if (!ok) input = '';
			});
		});
	});

	async function submit() {
		const q = input.trim();
		if (!q) return;
		if (!(await requireSupporter(() => void submit()))) return;
		searchQuery = q;
	}
	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			void submit();
		}
	}
	function clear() {
		input = '';
		searchQuery = null;
	}

	// Non-OK responses throw (status attached) so real failures surface as an
	// error state instead of a fake empty grid.
	async function fetchSavesPage(path: string): Promise<{ items: SaveView[]; cursor?: string }> {
		const res = await apiFetch(path);
		if (!res.ok) {
			const err = new Error(`request failed: ${res.status}`) as Error & { status?: number };
			err.status = res.status;
			throw err;
		}
		const data = await res.json();
		return { items: data.saves ?? [], cursor: data.cursor };
	}

	const feed = useInfiniteScroll<SaveView>(async (cursor) => {
		if (searchQuery) {
			const params = new URLSearchParams({ q: searchQuery, limit: '50' });
			for (const uri of scopeUris) params.append('collections', uri);
			if (cursor) params.set('cursor', cursor);
			return fetchSavesPage(`/xrpc/is.currents.feed.searchLibrarySaves?${params}`);
		}
		if (scopeUris.length > 0) {
			// Browsing scoped to collections: the backend filters by a single
			// collection at a time (getCollectionSaves), so page through the
			// selection sequentially — cursor = "<index>\n<inner cursor>",
			// skipping empty collections so a page always carries items when
			// more exist.
			const [idxStr, inner] = cursor ? cursor.split('\n') : ['0', ''];
			let idx = Number(idxStr);
			let innerCursor = inner;
			for (;;) {
				const params = new URLSearchParams({ collection: scopeUris[idx], limit: '50' });
				if (innerCursor) params.set('cursor', innerCursor);
				const page = await fetchSavesPage(`/xrpc/is.currents.feed.getCollectionSaves?${params}`);
				if (page.cursor) return { items: page.items, cursor: `${idx}\n${page.cursor}` };
				if (page.items.length > 0 || idx + 1 >= scopeUris.length) {
					return {
						items: page.items,
						cursor: idx + 1 < scopeUris.length ? `${idx + 1}\n` : undefined
					};
				}
				idx += 1;
				innerCursor = '';
			}
		}
		// No query, no scope: the whole library — every saved image, deduplicated.
		const params = new URLSearchParams({ limit: '50' });
		if (cursor) params.set('cursor', cursor);
		return fetchSavesPage(`/xrpc/is.currents.feed.getLibrarySaves?${params}`);
	});

	let errorStatus = $derived((feed.error as { status?: number } | null)?.status);
	let visible = $derived(feed.items.filter((i) => !shouldHide(i.labels)));

	// Reload whenever the query or scope changes.
	$effect(() => {
		void searchQuery;
		void scopeUris;
		untrack(() => {
			feed.reset();
			feed.loadMore();
		});
	});

	// Infinite scroll: the sentinel observed within the sidebar's own scroll
	// container (the viewport-rooted default never fires — the sentinel is
	// clipped by the container until it's actually scrolled into view).
	let scrollEl = $state<HTMLElement | null>(null);
	let sentinel = $state<HTMLDivElement>();
	$effect(() => {
		if (!sentinel || !scrollEl) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) feed.loadMore();
			},
			{ root: scrollEl, rootMargin: '600px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});
	// The observer only fires on intersection *changes*: a short page (e.g. a
	// small filtered collection) can leave the sentinel visible with nothing to
	// scroll, stalling the feed. After each load, keep going while the sentinel
	// is still within reach.
	$effect(() => {
		if (feed.loading || !feed.hasMore || feed.error || !sentinel || !scrollEl) return;
		if (sentinel.getBoundingClientRect().top < scrollEl.getBoundingClientRect().bottom + 600) {
			void feed.loadMore();
		}
	});

	let containerWidth = $state<number>();
	const gap = 8;
	let frameWidth = $derived(
		containerWidth !== undefined ? Math.max(90, Math.floor((containerWidth - gap - 2) / 2)) : 150
	);

	const skeletonShapes: Array<[number, number]> = [
		[3, 4],
		[2, 3],
		[4, 5],
		[3, 5],
		[4, 3],
		[5, 7],
		[2, 3],
		[3, 4]
	];
</script>

<Sidebar.Root collapsible="offcanvas" variant="inset" side="left">
	<Sidebar.Header class="gap-3 pb-1">
		<div class="px-1 pt-1">
			<a href="/canvas" class="block h-5 w-fit text-foreground" aria-label="Currents">
				<Logo />
			</a>
		</div>
		<ModeSwitcher mode="canvas" variant="sidebar" />
		<div class="flex items-center gap-2">
			<div class="relative flex-1">
				<SearchIcon
					class="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground"
				/>
				<Input
					bind:value={input}
					onkeydown={onKeydown}
					placeholder="Search your library…"
					class="h-9 bg-background pl-8 {input ? 'pr-8' : ''}"
					autocorrect="off"
					autocapitalize="off"
					spellcheck={false}
				/>
				{#if input}
					<button
						type="button"
						onclick={clear}
						class="absolute top-1/2 right-1.5 -translate-y-1/2 rounded p-1 text-muted-foreground hover:bg-muted"
						aria-label="Clear search"
					>
						<X class="size-3.5" />
					</button>
				{/if}
			</div>
			<Popover.Root>
				<Popover.Trigger>
					{#snippet child({ props })}
						<Button
							{...props}
							variant="ghost"
							size="icon"
							class="relative shrink-0"
							aria-label="Filter by collection"
						>
							<ListFilter />
							{#if scope.size > 0}
								<span
									class="absolute -top-0.5 -right-0.5 flex size-3.5 items-center justify-center rounded-full bg-primary text-[10px] leading-none font-semibold text-primary-foreground"
								>
									{scope.size}
								</span>
							{/if}
						</Button>
					{/snippet}
				</Popover.Trigger>
				<Popover.Content align="start" class="w-64 p-0">
					<Command.Root shouldFilter={false} class="bg-transparent">
						<Command.List>
							<CollectionFilterItems
								collections={collections.items}
								favourites={favouriteCollections.items}
								selected={scope}
								onToggle={toggleScope}
							/>
						</Command.List>
					</Command.Root>
				</Popover.Content>
			</Popover.Root>
		</div>
	</Sidebar.Header>

	<Sidebar.Content bind:ref={scrollEl} class="[overflow-anchor:none]">
		<div class="p-2">
			{#if feed.error && visible.length === 0}
				<div
					class="flex flex-col items-center justify-center gap-2 py-16 text-center text-sm text-muted-foreground"
				>
					<TriangleAlert class="size-6" />
					{#if errorStatus === 401}
						<p>Your session has expired.</p>
						<Button href="/login" variant="outline" size="sm" class="mt-1">Log in again</Button>
					{:else}
						<p>{searchQuery ? "Couldn't run the search." : "Couldn't load your images."}</p>
						<Button variant="outline" size="sm" class="mt-1" onclick={() => feed.retry()}>
							Try again
						</Button>
					{/if}
				</div>
			{:else if visible.length === 0 && !feed.loading}
				<div
					class="flex flex-col items-center justify-center gap-2 py-16 text-center text-sm text-muted-foreground"
				>
					<ImageOff class="size-6" />
					<p>
						{#if searchQuery}
							No results for “{searchQuery}”.
						{:else if scopeUris.length > 0}
							No images in these collections yet.
						{:else}
							You haven't saved any images yet.
						{/if}
					</p>
				</div>
			{:else}
				<div bind:clientWidth={containerWidth}>
					<BalancedMasonryGrid {frameWidth} {gap} style="overflow: visible;">
						{#each visible as item (item.uri)}
							{@const image = getImageContent(item)}
							<Frame width={image?.width ?? 3} height={image?.height ?? 4}>
								<div
									data-uri={item.uri}
									class="h-full w-full overflow-hidden rounded-lg"
									style={image?.dominantColor
										? `background-color: ${image.dominantColor}`
										: undefined}
								>
									<LabeledMedia labels={item.labels}>
										{#if image}
											<img
												src={image.imageUrl}
												alt={image.alt ?? item.text ?? ''}
												loading="lazy"
												class="w-full"
												style={image.width && image.height
													? `aspect-ratio: ${image.width} / ${image.height}`
													: undefined}
											/>
										{:else}
											<div
												class="flex items-center justify-center bg-muted text-xs text-muted-foreground"
												style="aspect-ratio: 3 / 4;"
											>
												Unsupported content
											</div>
										{/if}
									</LabeledMedia>
								</div>
							</Frame>
						{/each}
						{#if feed.loading}
							{#each skeletonShapes as [w, h], i (i)}
								<Frame width={w} height={h}>
									<Skeleton class="h-full w-full rounded-lg" />
								</Frame>
							{/each}
						{/if}
					</BalancedMasonryGrid>
				</div>
				{#if feed.error}
					<div class="flex items-center justify-center gap-3 py-4 text-sm text-muted-foreground">
						<span>Couldn't load more.</span>
						<Button variant="outline" size="sm" onclick={() => feed.retry()}>Try again</Button>
					</div>
				{:else if feed.hasMore}
					<div bind:this={sentinel} class="h-1"></div>
				{/if}
			{/if}
		</div>
	</Sidebar.Content>

	<Sidebar.Footer>
		<SidebarUserMenu />
	</Sidebar.Footer>
</Sidebar.Root>
