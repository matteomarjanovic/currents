<script lang="ts">
	import { untrack } from 'svelte';
	import { BalancedMasonryGrid, Frame } from '@masonry-grid/svelte';
	import { apiFetch } from '$lib/api';
	import { getImageContent, type SaveView } from '$lib/types';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import LabeledMedia from '$lib/components/labeled-media.svelte';
	import { shouldHide } from '$lib/stores/moderation-prefs.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import { useSidebar } from '$lib/components/ui/sidebar';
	import * as ContextMenu from '$lib/components/ui/context-menu';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Drawer from '$lib/components/ui/drawer';
	import { Button } from '$lib/components/ui/button';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import { emitSaveRemoved, onSaveRemoved } from '$lib/stores/save-events.svelte';
	import { copyLink, copyImage, downloadImage } from '$lib/save-actions';
	import { toast } from 'svelte-sonner';
	import ImageOff from '@lucide/svelte/icons/image-off';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import Scan from '@lucide/svelte/icons/scan';
	import Sparkles from '@lucide/svelte/icons/sparkles';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import Copy from '@lucide/svelte/icons/copy';
	import LinkIcon from '@lucide/svelte/icons/link';
	import Download from '@lucide/svelte/icons/download';
	import Trash2 from '@lucide/svelte/icons/trash-2';

	let {
		selectedUri = '',
		selectedSaveUri = null,
		onSelectSave,
		onFindSimilar,
		search = null,
		similar = null
	}: {
		selectedUri?: string;
		selectedSaveUri?: string | null;
		onSelectSave: (save: SaveView) => void;
		onFindSimilar: (save: SaveView) => void;
		search?: { query: string; collections: string[] } | null;
		similar?: { uri: string; collections: string[] } | null;
	} = $props();

	// The tile menu is shared between the right-click context menu and the options
	// button's dropdown. bits-ui's ContextMenu and DropdownMenu expose the same
	// Item/Sub/Separator API, so one snippet renders into either — passed the matching
	// namespace so each item wires up to its own parent menu.
	const dropdownMenu = DropdownMenu as unknown as typeof ContextMenu;

	const feed = useInfiniteScroll<SaveView>(async (cursor) => {
		if (similar) {
			const params = new URLSearchParams({ uri: similar.uri, limit: '50' });
			for (const uri of similar.collections) params.append('collections', uri);
			if (cursor) params.set('cursor', cursor);
			const res = await apiFetch(`/xrpc/is.currents.feed.findSimilarInLibrary?${params}`);
			if (!res.ok) return { items: [], cursor: undefined };
			const data = await res.json();
			return { items: data.saves ?? [], cursor: data.cursor };
		}
		if (search) {
			const params = new URLSearchParams({ q: search.query, limit: '50' });
			for (const uri of search.collections) params.append('collections', uri);
			if (cursor) params.set('cursor', cursor);
			const res = await apiFetch(`/xrpc/is.currents.feed.searchLibrarySaves?${params}`);
			if (!res.ok) return { items: [], cursor: undefined };
			const data = await res.json();
			return { items: data.saves ?? [], cursor: data.cursor };
		}
		if (selectedUri) {
			const params = new URLSearchParams({ collection: selectedUri, limit: '50' });
			if (cursor) params.set('cursor', cursor);
			const res = await apiFetch(`/xrpc/is.currents.feed.getCollectionSaves?${params}`);
			if (!res.ok) return { items: [], cursor: undefined };
			const data = await res.json();
			return { items: data.saves ?? [], cursor: data.cursor };
		}
		// No collection selected: the whole library — every saved image, deduplicated.
		const params = new URLSearchParams({ limit: '50' });
		if (cursor) params.set('cursor', cursor);
		const res = await apiFetch(`/xrpc/is.currents.feed.getLibrarySaves?${params}`);
		if (!res.ok) return { items: [], cursor: undefined };
		const data = await res.json();
		return { items: data.saves ?? [], cursor: data.cursor };
	});

	// Reload whenever the collection / search / find-similar source changes; there's
	// always something to load now (similar / search / collection / library).
	$effect(() => {
		void selectedUri;
		void search;
		void similar;
		untrack(() => {
			feed.reset();
			feed.loadMore();
		});
	});

	let visible = $derived(feed.items.filter((i) => !shouldHide(i.labels)));

	// ── Context-menu actions ──────────────────────────────────────────────────
	// Mobile "Add to collection" opens a shared drawer (desktop uses an inline submenu).
	let addTarget = $state<SaveView | null>(null);
	let addOpen = $state(false);
	function openAddDrawer(item: SaveView) {
		addTarget = item;
		addOpen = true;
	}
	function onItemSavesChange(item: SaveView, saves: { collectionUri: string; saveUri: string }[]) {
		item.viewer = { ...(item.viewer ?? {}), saves };
	}

	async function removeFromCollection(item: SaveView) {
		const rkey = item.uri.split('/').pop();
		feed.removeItem(item.uri); // optimistic
		try {
			const res = await apiFetch(`/save/${rkey}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(`${res.status}`);
			emitSaveRemoved({ saveUri: item.uri, collectionUri: selectedUri });
			toast.success('Removed from collection');
		} catch {
			toast.error('Could not remove from collection');
			feed.reset();
			feed.loadMore();
		}
	}

	// Drop a tile when its save is removed elsewhere (the detail sidebar's selector, or
	// this menu's inline "Add to collection" toggling the current collection off).
	$effect(() =>
		onSaveRemoved((e) => {
			if (selectedUri && e.collectionUri === selectedUri) feed.removeItem(e.saveUri);
		})
	);

	// Infinite scroll: observe a sentinel within this component's scroll container.
	let scrollEl = $state<HTMLDivElement>();
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

	let containerWidth = $state<number>();
	const gap = 12;
	let frameWidth = $derived(
		containerWidth !== undefined && containerWidth < 560
			? Math.max(120, Math.floor((containerWidth - gap - 2) / 2))
			: 200
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
		[3, 4]
	];

	// ── Scroll anchoring across reflows ───────────────────────────────────────
	// Opening/closing a sidebar changes the grid width, so the masonry reflows
	// (the column count changes) and the same scroll offset would show different
	// images. We pin a reference tile to its pre-reflow viewport offset for the
	// duration of the animation: the clicked image when the detail panel opens,
	// otherwise the top-left fully-visible image (on close or a left-sidebar toggle).
	const sidebar = useSidebar();
	let anchorRaf: number | undefined;

	function tileTop(el: HTMLElement, sc: HTMLElement): number {
		return el.getBoundingClientRect().top - sc.getBoundingClientRect().top;
	}
	function tileByUri(uri: string): HTMLElement | null {
		if (!scrollEl) return null;
		for (const el of scrollEl.querySelectorAll<HTMLElement>('[data-uri]')) {
			if (el.dataset.uri === uri) return el;
		}
		return null;
	}
	function topLeftVisibleUri(): string | null {
		if (!scrollEl) return null;
		const top = scrollEl.getBoundingClientRect().top;
		let best: { uri: string; top: number; left: number } | null = null;
		for (const el of scrollEl.querySelectorAll<HTMLElement>('[data-uri]')) {
			const r = el.getBoundingClientRect();
			if (r.top < top - 1) continue; // skip tiles cut off at the top
			if (!best || r.top < best.top - 1 || (r.top <= best.top + 1 && r.left < best.left)) {
				best = { uri: el.dataset.uri ?? '', top: r.top, left: r.left };
			}
		}
		return best?.uri || null;
	}
	// Pin a tile's top to `recorded` (its pre-reflow viewport offset) every frame
	// for a fixed window covering the width animation plus the masonry reflow that
	// can land just after it — no early exit, since the column-count change can
	// happen late in the animation.
	function startPin(el: HTMLElement, sc: HTMLElement, recorded: number) {
		if (anchorRaf) cancelAnimationFrame(anchorRaf);
		const start = performance.now();
		let stable = 0;
		const tick = () => {
			if (!el.isConnected) {
				anchorRaf = undefined;
				return;
			}
			const delta = tileTop(el, sc) - recorded;
			if (Math.abs(delta) >= 0.5) {
				sc.scrollTop += delta;
				stable = 0;
			} else {
				stable++;
			}
			// Run at least through the animation (so we don't stop before the
			// column-count reflow lands), then keep going until the layout has held
			// for a few frames; capped so we never run away.
			const elapsed = performance.now() - start;
			const done = elapsed >= 900 || (elapsed >= 300 && stable >= 5);
			anchorRaf = done ? undefined : requestAnimationFrame(tick);
		};
		anchorRaf = requestAnimationFrame(tick);
	}
	// Close / left-toggle: the current layout is the correct reference, so anchor
	// the top-left fully-visible tile now.
	function pinTopLeft() {
		const uri = topLeftVisibleUri();
		const el = uri ? tileByUri(uri) : null;
		if (el && scrollEl) startPin(el, scrollEl, tileTop(el, scrollEl));
	}
	// Open: capture the clicked tile's offset BEFORE opening — mounting the panel
	// reflows the grid immediately, so the post-open layout is already wrong.
	function selectTile(item: SaveView) {
		const el = scrollEl ? tileByUri(item.uri) : null;
		const recorded = el && scrollEl ? tileTop(el, scrollEl) : null;
		onSelectSave(item);
		if (el && scrollEl && recorded !== null) startPin(el, scrollEl, recorded);
	}

	// Closing and left-sidebar toggles are caught here; opening is handled in
	// selectTile (it needs the pre-open offset).
	let prevSelected: string | null = null;
	let prevSidebarState = sidebar.state;
	$effect(() => {
		const cur = selectedSaveUri ?? null;
		const state = sidebar.state;
		untrack(() => {
			if (cur !== prevSelected) {
				if (!cur && prevSelected) pinTopLeft();
				prevSelected = cur;
			}
			if (state !== prevSidebarState) {
				pinTopLeft();
				prevSidebarState = state;
			}
		});
	});
	$effect(() => () => {
		if (anchorRaf) cancelAnimationFrame(anchorRaf);
	});
</script>

{#snippet menuItems(Menu: typeof ContextMenu, item: SaveView)}
	<Menu.Item onSelect={() => selectTile(item)}>
		<Scan />
		Open
	</Menu.Item>
	<Menu.Item onSelect={() => onFindSimilar(item)}>
		<Sparkles />
		Find similar in library
	</Menu.Item>
	<Menu.Separator />
	{#if sidebar.isMobile}
		<Menu.Item onSelect={() => openAddDrawer(item)}>
			<FolderPlus />
			Add to collection…
		</Menu.Item>
	{:else}
		<Menu.Sub>
			<Menu.SubTrigger class="gap-2.5">
				<FolderPlus />
				Add to collection
			</Menu.SubTrigger>
			<!-- Scroll on an inner div so the SubContent's frosted background (tint +
			     backdrop-blur) stays fixed and covers every row; scrolling the panel
			     itself drags the blur layer away, leaving revealed rows see-through. -->
			<Menu.SubContent class="w-64 overflow-hidden p-0">
				<div class="max-h-[50vh] overflow-y-auto p-1.5">
					<CollectionSelector
						{item}
						variant="inline"
						onSavesChange={(saves) => onItemSavesChange(item, saves)}
					/>
				</div>
			</Menu.SubContent>
		</Menu.Sub>
	{/if}
	<Menu.Item onSelect={() => copyImage(item)}>
		<Copy />
		Copy image
	</Menu.Item>
	<Menu.Item onSelect={() => copyLink(item)}>
		<LinkIcon />
		Copy link
	</Menu.Item>
	<Menu.Item onSelect={() => downloadImage(item)}>
		<Download />
		Download
	</Menu.Item>
	{#if selectedUri && !search}
		<Menu.Separator />
		<Menu.Item variant="destructive" onSelect={() => removeFromCollection(item)}>
			<Trash2 />
			Remove from collection
		</Menu.Item>
	{/if}
{/snippet}

<!-- overflow-anchor:none disables the browser's native scroll anchoring, which
     misfires on the masonry's transform-based layout; we anchor manually instead. -->
<div bind:this={scrollEl} class="min-h-0 flex-1 overflow-y-auto p-4 [overflow-anchor:none]">
	{#if visible.length === 0 && !feed.loading}
		<div
			class="flex h-full flex-col items-center justify-center gap-2 text-center text-sm text-muted-foreground"
		>
			<ImageOff class="size-6" />
			<p>
				{#if similar}
					No similar images in your library.
				{:else if search}
					No results for “{search.query}”.
				{:else if selectedUri}
					No images in this collection yet.
				{:else}
					You haven't saved any images yet.
				{/if}
			</p>
		</div>
	{:else}
		<div bind:clientWidth={containerWidth}>
			<!-- The masonry sets the container to overflow:hidden + an explicit height,
			     which clips the hover ring on edge tiles (the bottom row especially).
			     Let it overflow so the 2px ring shows; the scroll container's p-4 gives
			     the ring room on every side. -->
			<BalancedMasonryGrid {frameWidth} {gap} style="overflow: visible;">
				{#each visible as item (item.uri)}
					{@const image = getImageContent(item)}
					<Frame width={image?.width ?? 3} height={image?.height ?? 4}>
						<div class="group relative h-full w-full">
							<ContextMenu.Root>
								<ContextMenu.Trigger>
									{#snippet child({ props })}
										<button
											{...props}
											type="button"
											data-uri={item.uri}
											onclick={() => selectTile(item)}
											class="relative block h-full w-full cursor-pointer overflow-hidden rounded-lg ring-primary transition-[box-shadow] group-hover:ring-2 {selectedSaveUri ===
											item.uri
												? 'ring-2 ring-primary'
												: ''}"
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
										</button>
									{/snippet}
								</ContextMenu.Trigger>
								<!-- overflow-*-visible so the "Add to collection" submenu (which renders
								     inside the content and opens to the side) isn't clipped by the
								     menu's default overflow-x-hidden/overflow-y-auto. -->
								<ContextMenu.Content class="w-56 overflow-x-visible overflow-y-visible">
									{@render menuItems(ContextMenu, item)}
								</ContextMenu.Content>
							</ContextMenu.Root>
							<!-- Options button: opens the same menu as right-click, anchored to the button.
							     Always visible on touch (no hover), revealed on hover on desktop, and kept
							     visible while its menu is open. -->
							<DropdownMenu.Root>
								<DropdownMenu.Trigger>
									{#snippet child({ props })}
										<Button
											{...props}
											variant="secondary"
											size="icon-sm"
											aria-label="Options"
											class="absolute right-2 bottom-2 aria-expanded:opacity-100 md:opacity-0 md:group-hover:opacity-100"
										>
											<Ellipsis />
										</Button>
									{/snippet}
								</DropdownMenu.Trigger>
								<DropdownMenu.Content
									align="end"
									class="w-56 overflow-x-visible overflow-y-visible"
								>
									{@render menuItems(dropdownMenu, item)}
								</DropdownMenu.Content>
							</DropdownMenu.Root>
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
		{#if feed.hasMore}
			<div bind:this={sentinel} class="h-1"></div>
		{/if}
	{/if}
</div>

<!-- Mobile "Add to collection": a shared bottom drawer (desktop uses the inline submenu). -->
<Drawer.Root bind:open={addOpen}>
	<Drawer.Content>
		<Drawer.Header>
			<Drawer.Title>Save to collection</Drawer.Title>
			<Drawer.Description>Pick one or more collections.</Drawer.Description>
		</Drawer.Header>
		<div class="max-h-[60vh] overflow-y-auto p-1.5">
			{#if addTarget}
				<CollectionSelector
					item={addTarget}
					variant="inline"
					onSavesChange={(saves) => addTarget && onItemSavesChange(addTarget, saves)}
				/>
			{/if}
		</div>
		<Drawer.Footer>
			<Drawer.Close>
				{#snippet child({ props })}
					<Button {...props} variant="outline">Done</Button>
				{/snippet}
			</Drawer.Close>
		</Drawer.Footer>
	</Drawer.Content>
</Drawer.Root>
