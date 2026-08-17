<script lang="ts">
	import { untrack } from 'svelte';
	import { BalancedMasonryGrid, Frame } from '@masonry-grid/svelte';
	import { apiFetch } from '$lib/api';
	import { resaveWithFallback } from '$lib/resave';
	import { getImageContent, type SaveView } from '$lib/types';
	import type { SvelteSet } from 'svelte/reactivity';
	import { selectedSaves, selectableUris, type BulkApi } from '$lib/organize-bulk';
	import { isCropped, tileRatio } from '$lib/image-ratio';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import LabeledMedia from '$lib/components/labeled-media.svelte';
	import SaveImage from '$lib/components/save-image.svelte';
	import { shouldHide } from '$lib/stores/moderation-prefs.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import { useSidebar } from '$lib/components/ui/sidebar';
	import * as ContextMenu from '$lib/components/ui/context-menu';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Drawer from '$lib/components/ui/drawer';
	import { Button } from '$lib/components/ui/button';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import { emitSaveRemoved, onSaveRemoved } from '$lib/stores/save-events.svelte';
	import { requireSupporter } from '$lib/stores/supporter.svelte';
	import { copyLink, copyImage, downloadImage } from '$lib/save-actions';
	import { toast } from 'svelte-sonner';
	import ImageOff from '@lucide/svelte/icons/image-off';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import Scan from '@lucide/svelte/icons/scan';
	import Sparkles from '@lucide/svelte/icons/sparkles';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import FolderInput from '@lucide/svelte/icons/folder-input';
	import Copy from '@lucide/svelte/icons/copy';
	import LinkIcon from '@lucide/svelte/icons/link';
	import Download from '@lucide/svelte/icons/download';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import SquareCheck from '@lucide/svelte/icons/square-check';
	import Check from '@lucide/svelte/icons/check';

	let {
		selectedUri = '',
		selectedSaveUri = null,
		onSelectSave,
		onFindSimilar,
		selectMode = $bindable(false),
		selected,
		bulk = $bindable(null),
		ownContext = true,
		search = null,
		similar = null,
		color = null
	}: {
		selectedUri?: string;
		selectedSaveUri?: string | null;
		onSelectSave: (save: SaveView) => void;
		onFindSimilar: (save: SaveView) => void;
		// Multi-select: the mode flag (owned by the page's header toggle) and the
		// shared set of selected save URIs.
		selectMode?: boolean;
		selected: SvelteSet<string>;
		// The bulk-action API, handed up so the page can render the action bar as a
		// sibling of the rounded panel rather than inside it. Everything it needs is
		// derived from the feed, which lives here.
		bulk?: BulkApi | null;
		// False only in a favourited (someone else's) collection — gates the actions
		// that write to the viewer's own records (move/label/attribution).
		ownContext?: boolean;
		search?: { query: string; collections: string[] } | null;
		similar?: { uri: string; collections: string[] } | null;
		color?: { hex: string; collections: string[] } | null;
	} = $props();

	// The tile menu is shared between the right-click context menu and the options
	// button's dropdown. bits-ui's ContextMenu and DropdownMenu expose the same
	// Item/Sub/Separator API, so one snippet renders into either — passed the matching
	// namespace so each item wires up to its own parent menu.
	const dropdownMenu = DropdownMenu as unknown as typeof ContextMenu;

	// Non-OK responses throw (status attached) so real failures surface as an
	// error state instead of a fake empty grid — a paid search answering "no
	// results" because the backend hiccupped would just look broken.
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
		if (color) {
			const params = new URLSearchParams({ color: color.hex, library: 'true', limit: '50' });
			// With a text query too it's a hybrid search: the color filters, the text orders.
			if (search) params.set('q', search.query);
			for (const uri of color.collections) params.append('collections', uri);
			if (cursor) params.set('cursor', cursor);
			return fetchSavesPage(`/xrpc/is.currents.feed.searchSavesByColor?${params}`);
		}
		if (similar) {
			const params = new URLSearchParams({ uri: similar.uri, limit: '50' });
			for (const uri of similar.collections) params.append('collections', uri);
			if (cursor) params.set('cursor', cursor);
			return fetchSavesPage(`/xrpc/is.currents.feed.findSimilarInLibrary?${params}`);
		}
		if (search) {
			const params = new URLSearchParams({ q: search.query, limit: '50' });
			for (const uri of search.collections) params.append('collections', uri);
			if (cursor) params.set('cursor', cursor);
			return fetchSavesPage(`/xrpc/is.currents.feed.searchLibrarySaves?${params}`);
		}
		if (selectedUri) {
			const params = new URLSearchParams({ collection: selectedUri, limit: '50' });
			if (cursor) params.set('cursor', cursor);
			return fetchSavesPage(`/xrpc/is.currents.feed.getCollectionSaves?${params}`);
		}
		// No collection selected: the whole library — every saved image, deduplicated.
		const params = new URLSearchParams({ limit: '50' });
		if (cursor) params.set('cursor', cursor);
		return fetchSavesPage(`/xrpc/is.currents.feed.getLibrarySaves?${params}`);
	});

	let errorStatus = $derived((feed.error as { status?: number } | null)?.status);

	// Reload whenever the collection / search / find-similar source changes; there's
	// always something to load now (similar / search / collection / library).
	$effect(() => {
		void selectedUri;
		void search;
		void similar;
		void color;
		untrack(() => {
			feed.reset();
			feed.loadMore();
		});
	});

	let visible = $derived(feed.items.filter((i) => !shouldHide(i.labels)));

	// ── Multi-select ──────────────────────────────────────────────────────────
	// The mode flag and the selected URI set are owned by the page (its header
	// toggle flips the mode); the canvas resolves them against the live feed and
	// runs the bulk actions that touch the feed (copy/move). The others live in
	// the action bar.
	let selectedList = $derived(selectedSaves(feed.items, selected));
	let selectableCount = $derived(selectableUris(visible).length);
	let canMove = $derived(ownContext && !!selectedUri && !search && !color && !similar);

	function toggleSelect(item: SaveView) {
		if (!getImageContent(item)) return; // unsupported content isn't selectable
		if (selected.has(item.uri)) selected.delete(item.uri);
		else selected.add(item.uri);
	}
	function selectAllLoaded() {
		for (const uri of selectableUris(visible)) selected.add(uri);
	}
	function enterSelectWith(item: SaveView) {
		selectMode = true;
		selected.clear();
		toggleSelect(item);
	}
	function exitSelect() {
		selected.clear();
		selectMode = false;
	}

	// Run an async op over `items` with at most `limit` in flight — copy/move each
	// do a PDS write per item, so a modest cap keeps the write budget in check.
	async function runBounded<T>(items: T[], limit: number, fn: (t: T) => Promise<void>) {
		const queue = [...items];
		const worker = async () => {
			for (;;) {
				const it = queue.shift();
				if (it === undefined) return;
				await fn(it);
			}
		};
		await Promise.all(Array.from({ length: Math.min(limit, queue.length) }, worker));
	}

	async function bulkCopy(dest: string) {
		const targets = selectedList;
		let ok = 0;
		let failed = 0;
		await runBounded(targets, 4, async (item) => {
			try {
				const res = await resaveWithFallback(item.uri, dest);
				if (!res.ok) throw new Error(`${res.status}`);
				ok++;
			} catch {
				failed++;
			}
		});
		if (ok > 0) toast.success(`Copied ${ok}${failed ? ` · ${failed} failed` : ''}`);
		else toast.error('Could not copy');
		exitSelect();
	}

	async function bulkMove(dest: string) {
		if (dest === selectedUri) {
			exitSelect();
			return;
		}
		const targets = selectedList;
		let ok = 0;
		let failed = 0;
		await runBounded(targets, 4, async (item) => {
			const rkey = item.uri.split('/').pop();
			const alreadyInDest = item.viewer?.saves?.some((s) => s.collectionUri === dest);
			try {
				if (!alreadyInDest) {
					const res = await resaveWithFallback(item.uri, dest);
					if (!res.ok) throw new Error(`resave: ${res.status}`);
				}
				const del = await apiFetch(`/api/save/${rkey}`, { method: 'DELETE' });
				if (!del.ok) throw new Error(`delete: ${del.status}`);
				feed.removeItem(item.uri);
				emitSaveRemoved({ saveUri: item.uri, collectionUri: selectedUri });
				ok++;
			} catch {
				failed++;
			}
		});
		if (ok > 0) toast.success(`Moved ${ok}${failed ? ` · ${failed} failed` : ''}`);
		else toast.error('Could not move');
		exitSelect();
	}

	// Hand the bulk API up once, as getters — the values behind them are $derived,
	// so the object stays live without an effect re-assigning it.
	bulk = {
		get saves() {
			return selectedList;
		},
		get selectableCount() {
			return selectableCount;
		},
		get canMove() {
			return canMove;
		},
		onSelectAll: selectAllLoaded,
		onClear: () => selected.clear(),
		onExit: exitSelect,
		onCopy: bulkCopy,
		onMove: bulkMove
	};

	// ── Context-menu actions ──────────────────────────────────────────────────
	// Mobile "Copy/Move to collection" opens a shared drawer (desktop uses an inline
	// submenu). Copy shows the multi-select membership list; move shows a destination
	// picker.
	let drawerTarget = $state<SaveView | null>(null);
	let drawerOpen = $state(false);
	let drawerMode = $state<'copy' | 'move'>('copy');
	function openCopyDrawer(item: SaveView) {
		drawerTarget = item;
		drawerMode = 'copy';
		drawerOpen = true;
	}
	function openMoveDrawer(item: SaveView) {
		drawerTarget = item;
		drawerMode = 'move';
		drawerOpen = true;
	}
	function onItemSavesChange(item: SaveView, saves: { collectionUri: string; saveUri: string }[]) {
		item.viewer = { ...(item.viewer ?? {}), saves };
	}

	async function removeFromCollection(item: SaveView) {
		const rkey = item.uri.split('/').pop();
		feed.removeItem(item.uri); // optimistic
		try {
			const res = await apiFetch(`/api/save/${rkey}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(`${res.status}`);
			emitSaveRemoved({ saveUri: item.uri, collectionUri: selectedUri });
			toast.success('Removed from collection');
		} catch {
			toast.error('Could not remove from collection');
			feed.reset();
			feed.loadMore();
		}
	}

	// Move relocates this save's record to another collection ('' = unsorted/profile).
	// It resaves into the destination first — so CreateResave inherits the original
	// createdAt from the still-present source — then deletes the source. If the image
	// is already in the destination, a resave would duplicate it, so just drop the
	// source instead.
	async function moveToCollection(item: SaveView, collectionUri: string) {
		if (collectionUri === selectedUri) return; // already here
		const rkey = item.uri.split('/').pop();
		const alreadyInDest = item.viewer?.saves?.some((s) => s.collectionUri === collectionUri);
		feed.removeItem(item.uri); // optimistic: leaves the current collection grid
		try {
			if (!alreadyInDest) {
				const res = await resaveWithFallback(item.uri, collectionUri);
				if (!res.ok) throw new Error(`resave: ${res.status}`);
			}
			const del = await apiFetch(`/api/save/${rkey}`, { method: 'DELETE' });
			if (!del.ok) throw new Error(`delete: ${del.status}`);
			emitSaveRemoved({ saveUri: item.uri, collectionUri: selectedUri });
			toast.success('Moved');
		} catch {
			toast.error('Could not move');
			feed.reset();
			feed.loadMore();
		}
	}

	// Drop a tile when its save is removed elsewhere (the detail sidebar's selector, or
	// this menu's inline "Copy to collection" toggling the current collection off).
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
	<Menu.Item onSelect={() => enterSelectWith(item)}>
		<SquareCheck />
		Select
	</Menu.Item>
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
		<Menu.Item onSelect={() => openCopyDrawer(item)}>
			<FolderPlus />
			Copy to collection…
		</Menu.Item>
	{:else}
		<Menu.Sub>
			<Menu.SubTrigger class="gap-2.5">
				<FolderPlus />
				Copy to collection
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
	<!-- Move: only when viewing a real collection, where this tile's record
	     unambiguously belongs to it. Destination picker (no item → pickerMode);
	     the Profile row selects '' = unsorted. -->
	{#if selectedUri && !search && !color && !similar}
		{#if sidebar.isMobile}
			<Menu.Item onSelect={() => openMoveDrawer(item)}>
				<FolderInput />
				Move to collection…
			</Menu.Item>
		{:else}
			<Menu.Sub>
				<Menu.SubTrigger class="gap-2.5">
					<FolderInput />
					Move to collection
				</Menu.SubTrigger>
				<Menu.SubContent class="w-64 overflow-hidden p-0">
					<div class="max-h-[50vh] overflow-y-auto p-1.5">
						<CollectionSelector variant="inline" onSelect={(uri) => moveToCollection(item, uri)} />
					</div>
				</Menu.SubContent>
			</Menu.Sub>
		{/if}
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
	{#if selectedUri && !search && !color && !similar}
		<Menu.Separator />
		<Menu.Item variant="destructive" onSelect={() => removeFromCollection(item)}>
			<Trash2 />
			Remove from collection
		</Menu.Item>
	{/if}
{/snippet}

{#snippet tileMedia(
	item: SaveView,
	image: ReturnType<typeof getImageContent>,
	ratio: { width: number; height: number }
)}
	<LabeledMedia labels={item.labels}>
		{#if image}
			<SaveImage
				{image}
				variant="grid"
				alt={image.alt ?? item.text ?? ''}
				class="w-full {isCropped(image.width, image.height) ? 'object-cover object-top' : ''}"
				wrapperClass="block w-full"
				overlayFit={isCropped(image.width, image.height)
					? 'object-cover object-top'
					: 'object-contain'}
				style={image.width && image.height
					? `aspect-ratio: ${ratio.width} / ${ratio.height}`
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
{/snippet}

<!-- overflow-anchor:none disables the browser's native scroll anchoring, which
     misfires on the masonry's transform-based layout; we anchor manually instead. -->
<div bind:this={scrollEl} class="min-h-0 flex-1 overflow-y-auto p-4 [overflow-anchor:none]">
	{#if feed.error && visible.length === 0}
		<div
			class="flex h-full flex-col items-center justify-center gap-2 text-center text-sm text-muted-foreground"
		>
			<TriangleAlert class="size-6" />
			{#if errorStatus === 401}
				<p>Your session has expired.</p>
				<Button href="/login" variant="outline" size="sm" class="mt-1">Log in again</Button>
			{:else if errorStatus === 403}
				<!-- Reachable by revisiting a `?color=` URL after the free trial colors
				     are spent — the gate runs before navigating, not on a cold load. -->
				<p>
					{#if color}
						You’ve used your free color searches.
					{:else}
						This is a supporter feature.
					{/if}
				</p>
				<Button variant="outline" size="sm" class="mt-1" onclick={() => void requireSupporter()}>
					Become a supporter
				</Button>
			{:else}
				<p>
					{#if search || color}
						Couldn't run the search.
					{:else if similar}
						Couldn't load similar images.
					{:else}
						Couldn't load your images.
					{/if}
				</p>
				<Button variant="outline" size="sm" class="mt-1" onclick={() => feed.retry()}>
					Try again
				</Button>
			{/if}
		</div>
	{:else if visible.length === 0 && !feed.loading}
		<div
			class="flex h-full flex-col items-center justify-center gap-2 text-center text-sm text-muted-foreground"
		>
			<ImageOff class="size-6" />
			<p>
				{#if color && search}
					No “{search.query}” images in this color. Try a broader color or drop the text.
				{:else if color}
					No images matching this color in your library.
				{:else if similar}
					No similar images in your library.
				{:else if search}
					No results for “{search.query}”.
				{:else if selectedUri}
					No images in this collection yet.
				{:else}
					You haven't saved any images yet.
				{/if}
			</p>
			{#if !similar && !search && !color && !selectedUri}
				<Button href="/explore" variant="outline" size="sm" class="mt-1">Go to explore mode</Button>
			{/if}
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
					{@const ratio = tileRatio(image?.width, image?.height)}
					<Frame width={ratio.width} height={ratio.height}>
						<div class="group relative h-full w-full">
							{#if selectMode}
								{@const isSel = selected.has(item.uri)}
								{@const ok = !!image}
								<button
									type="button"
									data-uri={item.uri}
									disabled={!ok}
									aria-pressed={isSel}
									onclick={() => toggleSelect(item)}
									class="relative block h-full w-full overflow-hidden rounded-lg {ok
										? 'cursor-pointer'
										: 'cursor-not-allowed'}"
									style={image?.dominantColor
										? `background-color: ${image.dominantColor}`
										: undefined}
								>
									{@render tileMedia(item, image, ratio)}
									{#if ok}
										<div
											class="absolute inset-0 rounded-lg transition-colors {isSel
												? 'bg-primary/20 ring-2 ring-primary ring-inset'
												: 'group-hover:bg-black/10'}"
										></div>
										<div
											class="absolute top-2 left-2 flex size-6 items-center justify-center rounded-full border-2 transition-colors {isSel
												? 'border-primary bg-primary text-primary-foreground'
												: 'border-white/90 bg-black/30'}"
										>
											{#if isSel}<Check class="size-4" />{/if}
										</div>
									{/if}
								</button>
							{:else}
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
												{@render tileMedia(item, image, ratio)}
											</button>
										{/snippet}
									</ContextMenu.Trigger>
									<!-- overflow-*-visible so the collection submenus (which render
									     inside the content and open to the side) aren't clipped by the
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
							{/if}
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
			<!-- A later page failed while earlier results are showing. -->
			<div class="flex items-center justify-center gap-3 py-6 text-sm text-muted-foreground">
				<span>Couldn't load more.</span>
				<Button variant="outline" size="sm" onclick={() => feed.retry()}>Try again</Button>
			</div>
		{:else if feed.hasMore}
			<div bind:this={sentinel} class="h-1"></div>
		{/if}
	{/if}
</div>

<!-- Mobile "Copy/Move to collection": a shared bottom drawer (desktop uses the inline submenu). -->
<Drawer.Root bind:open={drawerOpen}>
	<Drawer.Content>
		<Drawer.Header>
			<Drawer.Title
				>{drawerMode === 'move' ? 'Move to collection' : 'Copy to collection'}</Drawer.Title
			>
			<Drawer.Description>
				{drawerMode === 'move' ? 'Pick a destination collection.' : 'Pick one or more collections.'}
			</Drawer.Description>
		</Drawer.Header>
		<div class="max-h-[60vh] overflow-y-auto p-1.5">
			{#if drawerTarget}
				{#if drawerMode === 'move'}
					<CollectionSelector
						variant="inline"
						onSelect={(uri) => {
							if (drawerTarget) moveToCollection(drawerTarget, uri);
							drawerOpen = false;
						}}
					/>
				{:else}
					<CollectionSelector
						item={drawerTarget}
						variant="inline"
						onSavesChange={(saves) => drawerTarget && onItemSavesChange(drawerTarget, saves)}
					/>
				{/if}
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
