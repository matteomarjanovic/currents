<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { apiFetch } from '$lib/api';
	import { getImageContent, type SaveView } from '$lib/types';
	import { shouldHide } from '$lib/stores/moderation-prefs.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import { neighbourOf } from '$lib/save-sequence';
	import { onBackButton } from '$lib/back-button';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import SaveImage from '$lib/components/save-image.svelte';
	import LabeledMedia from '$lib/components/labeled-media.svelte';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import ImageActionMenu from '$lib/components/image-action-menu.svelte';
	import ExploreTrail from '$lib/components/explore-trail.svelte';
	import ImageSwipeStage from './image-swipe-stage.svelte';
	import MobileRelatedHistory from './mobile-related-history.svelte';
	import { Button } from '$lib/components/ui/button';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import ExternalLink from '@lucide/svelte/icons/external-link';

	let {
		save,
		mainPrevious,
		mainNext,
		onMainNavigate,
		onContinueFullScreenPinch,
		active = true,
		onViewFullScreen,
		onSavesChange
	}: {
		save: SaveView;
		mainPrevious: SaveView | null;
		mainNext: SaveView | null;
		onMainNavigate: (direction: -1 | 1) => void;
		onContinueFullScreenPinch: (
			save: SaveView,
			pointers: { id: number; x: number; y: number }[]
		) => void;
		active?: boolean;
		onViewFullScreen: (save: SaveView) => void;
		onSavesChange?: (saves: { collectionUri: string; saveUri: string }[]) => void;
	} = $props();

	type Entry = {
		save: SaveView;
		sequence: SaveView[];
		page?: { items: SaveView[]; cursor?: string };
		scrollTop: number;
	};
	// Retain data and scroll positions, with just one interactive detail/grid mounted.
	let entries = $state<Entry[]>([]);
	let index = $state(0);
	let current = $derived(entries[index]?.save ?? save);
	let image = $derived(getImageContent(current));
	let trail = $derived(entries.slice(0, index + 1).map((entry) => entry.save));
	let sequence = $derived(
		(entries[index]?.sequence ?? []).filter((item) => !shouldHide(item.labels))
	);
	let swipePrevious = $derived(index > 0 ? neighbourOf(sequence, current.uri, -1) : mainPrevious);
	let swipeNext = $derived(index > 0 ? neighbourOf(sequence, current.uri, 1) : mainNext);
	function navigateSwipe(direction: -1 | 1) {
		if (!handleImageArrow(direction)) onMainNavigate(direction);
	}
	const mobile = new IsMobile();

	const related = useInfiniteScroll<SaveView>(async (cursor) => {
		const params = new URLSearchParams({ uri: current.uri, limit: cursor ? '50' : '12' });
		if (cursor) params.set('cursor', cursor);
		const res = await apiFetch(`/xrpc/is.currents.feed.getRelatedSaves?${params}`);
		if (!res.ok) throw new Error(`request failed: ${res.status}`);
		const data = await res.json();
		return { items: data.saves ?? [], cursor: data.cursor };
	});
	let visible = $derived(related.items.filter((item) => !shouldHide(item.labels)));
	let scrollEl = $state<HTMLDivElement>();
	let hero = $state<HTMLDivElement>();
	let sentinel = $state<HTMLDivElement>();
	let currentHidden = $state(false);
	let preview = $state<{ uri: string; src: string } | null>(null);
	let loadedImageUrl = $state('');
	let previewSrc = $derived(
		preview?.uri === current.uri && loadedImageUrl !== image?.imageUrl ? preview.src : null
	);
	let restoreVersion = 0;
	let restoring = $state(false);

	function capturePreview(uri: string) {
		const thumbnail = document.querySelector<HTMLImageElement>(
			`[data-save-uri="${CSS.escape(uri)}"] img, [data-uri="${CSS.escape(uri)}"] img`
		);
		preview =
			thumbnail?.complete && thumbnail.naturalWidth ? { uri, src: thumbnail.currentSrc } : null;
	}

	async function restoreScroll(top: number) {
		const version = ++restoreVersion;
		restoring = true;
		await tick();
		// A nonzero position needs masonry's restored geometry. At the top, finish
		// immediately so a user's first scroll cannot be overwritten by a later frame.
		if (top > 0) {
			await new Promise<void>((resolve) =>
				requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
			);
		}
		if (version === restoreVersion && active && scrollEl) {
			scrollEl.scrollTop = top;
			restoring = false;
		}
	}

	function cacheCurrent() {
		const entry = entries[index];
		if (!entry) return;
		if (!restoring) entry.scrollTop = scrollEl?.scrollTop ?? entry.scrollTop;
		if (related.items.length || (!related.loading && !related.error)) {
			entry.page = { items: related.items, cursor: related.cursor };
		}
	}

	function showEntry() {
		const top = entries[index]?.scrollTop ?? 0;
		currentHidden = false;
		restoring = true;
		if (scrollEl) scrollEl.scrollTop = 0;
		related.reset(entries[index]?.page);
		if (!entries[index]?.page && active) void related.loadMore();
		void restoreScroll(top);
	}

	function openRelated(target: SaveView, items: SaveView[]) {
		cacheCurrent();
		capturePreview(target.uri);
		entries = [...entries.slice(0, index + 1), { save: target, sequence: items, scrollTop: 0 }];
		index = entries.length - 1;
		showEntry();
	}

	function visit(target: number) {
		if (target === index) return scrollToImage();
		if (target < 0 || target >= entries.length) return;
		cacheCurrent();
		capturePreview(entries[target].save.uri);
		index = target;
		showEntry();
	}

	function revisitFromMobileTrail(target: number) {
		visit(target);
		entries = entries.slice(0, index + 1);
	}

	// Return whether this journey owns the arrow, even at a sibling-list boundary.
	export function handleImageArrow(direction: -1 | 1): boolean {
		if (index === 0) return false;
		const target = neighbourOf(sequence, current.uri, direction);
		if (!target) return true;
		capturePreview(target.uri);
		// Like Explore's arrow navigation, change the current step without growing the trail.
		entries[index] = { save: target, sequence: entries[index].sequence, scrollTop: 0 };
		showEntry();
		return true;
	}

	function scrollToImage() {
		scrollEl?.scrollTo({ top: 0, behavior: prefersReducedMotion.current ? 'instant' : 'smooth' });
	}

	function updateSaves(saves: { collectionUri: string; saveUri: string }[]) {
		current.viewer = { ...current.viewer, saves };
		if (current.uri === save.uri) onSavesChange?.(saves);
	}

	$effect.pre(() => {
		if (!active) untrack(cacheCurrent);
	});

	let rootUri = '';
	$effect(() => {
		if (save.uri === rootUri) return;
		rootUri = save.uri;
		untrack(() => {
			capturePreview(save.uri);
			entries = [{ save, sequence: [], scrollTop: 0 }];
			index = 0;
			showEntry();
		});
	});
	$effect(() => {
		if (!active || !scrollEl) return;
		untrack(() => {
			if (!entries[index]?.page && !related.items.length) void related.loadMore();
			void restoreScroll(entries[index]?.scrollTop ?? 0);
		});
	});
	$effect(() => {
		if (!active || !hero || !scrollEl) return;
		void current.uri;
		const root = scrollEl;
		const observer = new IntersectionObserver(
			([entry]) => {
				currentHidden =
					!entry.isIntersecting &&
					entry.boundingClientRect.bottom <= root.getBoundingClientRect().top;
			},
			{ root }
		);
		observer.observe(hero);
		return () => observer.disconnect();
	});
	$effect(() => {
		if (!active || restoring || related.loading || !sentinel || !scrollEl) return;
		const observer = new IntersectionObserver(
			([entry]) => {
				if (entry.isIntersecting) void related.loadMore();
			},
			{ root: scrollEl, rootMargin: '1200px 0px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});
	$effect(() => {
		if (active && index > 0) return onBackButton(() => visit(index - 1));
	});

	let source = $derived.by(() => {
		try {
			const url = new URL(current.originUrl ?? '');
			return ['http:', 'https:'].includes(url.protocol) ? url : null;
		} catch {
			return null;
		}
	});
</script>

<div data-related-browser class="relative flex h-full min-h-0 flex-col">
	{#if active}
		{#if !mobile.current}
			<div
				data-related-toolbar
				class="relative z-20 flex h-11 shrink-0 items-center gap-1 px-3 md:px-4"
			>
				<div class="flex items-center gap-0.5" role="group" aria-label="Related navigation">
					<Button
						variant="ghost"
						size="icon-sm"
						disabled={index === 0}
						onclick={() => visit(index - 1)}
						aria-label="Back in related images"
						title="Back"><ArrowLeft class="size-4" /></Button
					>
					<Button
						variant="ghost"
						size="icon-sm"
						disabled={index === entries.length - 1}
						onclick={() => visit(index + 1)}
						aria-label="Forward in related images"
						title="Forward"><ArrowRight class="size-4" /></Button
					>
				</div>
				<div class="pointer-events-none absolute -top-2.5 right-3 z-30 md:right-4">
					<ExploreTrail
						saves={trail}
						{currentHidden}
						onCurrent={scrollToImage}
						onVisit={visit}
						vertical
					/>
				</div>
			</div>
		{/if}
		<div
			bind:this={scrollEl}
			data-related-scroll
			style:padding-bottom={mobile.current ? 'calc(env(safe-area-inset-bottom) + 6rem)' : undefined}
			class="min-h-0 flex-1 overflow-y-auto p-3 pt-1 [overflow-anchor:none] md:px-4"
			onscroll={() => {
				if (!restoring && entries[index]) entries[index].scrollTop = scrollEl?.scrollTop ?? 0;
			}}
		>
			<section data-related-current={current.uri} class="mb-5 flex flex-col gap-3">
				<div bind:this={hero} class="flex justify-center">
					<ImageSwipeStage
						save={current}
						previous={swipePrevious}
						next={swipeNext}
						enabled={mobile.current && active}
						onNavigate={navigateSwipe}
						onContinuePinch={(pointers) => onContinueFullScreenPinch(current, pointers)}
					>
						{#if shouldHide(current.labels)}
							<div
								class="flex h-48 w-full items-center justify-center bg-muted text-sm text-muted-foreground"
							>
								Hidden by your settings
							</div>
						{:else if image}
							<LabeledMedia labels={current.labels} class="flex w-full justify-center">
								<button
									type="button"
									data-swipe-image
									aria-label="View selected image full screen"
									class="flex w-full cursor-zoom-in justify-center"
									onclick={() => onViewFullScreen(current)}
								>
									<div
										class="overflow-hidden bg-muted"
										data-related-preview={previewSrc ? '' : undefined}
										style={`width: min(100%, ${12 * (image.width && image.height ? image.width / image.height : 1)}rem); max-width: 18rem; aspect-ratio: ${image.width ?? 1} / ${image.height ?? 1};${previewSrc ? ` background: url(${JSON.stringify(previewSrc)}) center / contain no-repeat;` : ''}`}
									>
										<SaveImage
											{image}
											alt={image.alt ?? current.text ?? ''}
											sizes="288px"
											loading="eager"
											wrapperClass="h-full w-full"
											class="max-h-48 w-full object-contain {loadedImageUrl !== image.imageUrl
												? 'opacity-0'
												: ''}"
											onLoad={() => (loadedImageUrl = image?.imageUrl ?? '')}
										/>
									</div>
								</button>
							</LabeledMedia>
						{/if}
					</ImageSwipeStage>
				</div>
				<div class="mx-auto flex w-full max-w-sm flex-col gap-3">
					<div class="flex items-center justify-between gap-2">
						<div class="min-w-0 flex-1">
							<CollectionSelector
								item={current}
								variant={mobile.current ? 'drawer' : 'popover'}
								onSavesChange={updateSaves}
							/>
						</div>
						<ImageActionMenu item={current} variant="dropdown" {onViewFullScreen} />
					</div>
					<div class="flex min-w-0 flex-col gap-1 text-xs text-muted-foreground">
						<span class="truncate">@{current.author.handle}</span>
						{#if source}<a
								href={source.href}
								target="_blank"
								rel="noopener noreferrer"
								class="inline-flex w-fit max-w-full items-center gap-1 hover:text-foreground"
								><span class="truncate">{source.hostname}</span><ExternalLink
									class="size-3 shrink-0"
								/></a
							>{/if}
					</div>
				</div>
				{#if current.text}<p class="text-sm leading-relaxed break-words whitespace-pre-wrap">
						{current.text}
					</p>{/if}
			</section>
			<h3 class="mb-3 text-sm font-medium">Related images</h3>
			{#if related.error && !visible.length}
				<div
					class="flex flex-col items-center gap-2 py-8 text-center text-sm text-muted-foreground"
				>
					<p>Couldn't load related images.</p>
					<Button variant="outline" size="sm" onclick={related.retry}>Try again</Button>
				</div>
			{:else if !visible.length && !related.loading}
				<p class="py-8 text-center text-sm text-muted-foreground">No similar images found.</p>
			{:else}
				<MasonryGrid
					items={related.items}
					loading={related.loading}
					linkToDetail={false}
					longPressSave
					{onViewFullScreen}
					onSelectSave={openRelated}
					reservePageScrollbarGutter={false}
					allowSingleColumn
				/>
				{#if related.error}<Button variant="outline" size="sm" class="my-3" onclick={related.retry}
						>Retry loading images</Button
					>{/if}
			{/if}
			{#if related.hasMore}<div bind:this={sentinel} class="h-1"></div>{/if}
		</div>
		{#if mobile.current}
			<MobileRelatedHistory
				saves={trail}
				currentIndex={index}
				canGoForward={index < entries.length - 1}
				onVisit={visit}
				onRevisit={revisitFromMobileTrail}
			/>
		{/if}
	{/if}
</div>
