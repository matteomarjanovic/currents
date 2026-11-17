<script lang="ts">
	import { page } from '$app/state';
	import { fly } from 'svelte/transition';
	import { prefersReducedMotion } from 'svelte/motion';
	import { getImageContent } from '$lib/types';
	import { effectiveVisibility, prefsPending } from '$lib/stores/moderation-prefs.svelte';
	import SaveImage from '$lib/components/save-image.svelte';
	import LabeledMedia from '$lib/components/labeled-media.svelte';
	import * as HoverCard from '$lib/components/ui/hover-card';

	let { currentHidden, onCurrent }: { currentHidden: boolean; onCurrent: () => void } = $props();
	let stack = $derived(page.state.saveStack ?? (page.state.save ? [page.state.save] : []));
	let entries = $derived(currentHidden ? stack : stack.slice(0, -1));
	let thumbnails = $derived(
		entries.map((save) => {
			const image = getImageContent(save);
			const ratio =
				image?.width && image.height ? Math.max(0.5, Math.min(2, image.width / image.height)) : 1;
			const width = Math.min(46 * ratio, 46);
			return { save, image, width, height: width / ratio };
		})
	);
	let availableWidth = $state(0);
	let hovered = $state(false);
	let focused = $state(false);
	let hoverIndex = $state<number | null>(null);
	let focusIndex = $state<number | null>(null);
	let previewIndex = $state<number | null>(null);
	$effect(() => {
		if (previewIndex !== null && previewIndex >= entries.length) previewIndex = null;
	});
	let open = $derived(hovered || focused);
	let activeIndex = $derived(hovered ? hoverIndex : focused ? focusIndex : previewIndex);
	let gap = $derived(thumbnails.length === 2 ? 20 : 12);
	let expandedWidth = $derived(
		Math.min(
			availableWidth,
			Math.max(
				76,
				thumbnails.reduce((sum, t) => sum + t.width, 24) + Math.max(0, thumbnails.length - 1) * gap
			)
		)
	);

	// Keep the trail inside the real gap, including Windows' viewport scrollbar.
	function measureSpace(node: HTMLElement) {
		const root = node.closest<HTMLElement>('[data-save-detail-overlay]') ?? document.body;
		const measure = () => {
			const actions =
				root.querySelector<HTMLElement>(
					'[data-save-detail-overlay][data-active="true"] [data-desktop-actions]'
				) ?? root.querySelector<HTMLElement>('[data-desktop-actions]');
			const right =
				actions?.getBoundingClientRect().left ?? document.documentElement.clientWidth - 16;
			availableWidth = Math.max(0, right - node.getBoundingClientRect().left - 12);
			if (actions) resize.observe(actions);
		};
		const resize = new ResizeObserver(measure);
		const mutations = new MutationObserver(measure);
		resize.observe(root);
		mutations.observe(root, { childList: true, subtree: true });
		window.addEventListener('resize', measure);
		measure();
		return () => {
			resize.disconnect();
			mutations.disconnect();
			window.removeEventListener('resize', measure);
		};
	}

	function positions(focus: number | null) {
		if (!thumbnails.length) return [];
		let offset = 0;
		const natural = thumbnails.map((t) => {
			const center = offset + t.width / 2;
			offset += t.width + gap;
			return center;
		});
		const first = thumbnails[0];
		const last = thumbnails[thumbnails.length - 1];
		const span = Math.max(0, expandedWidth - 24 - first.width / 2 - last.width / 2);
		const centers = natural.map((center) =>
			thumbnails.length === 1
				? expandedWidth / 2
				: 12 +
					first.width / 2 +
					((center - natural[0]) / (natural[natural.length - 1] - natural[0])) * span
		);
		const lo = focus === null ? 0 : Math.floor(focus);
		const hi = Math.min(lo + 1, centers.length - 1);
		const pointer =
			focus === null ? null : centers[lo] + (centers[hi] - centers[lo]) * (focus - lo);
		const maxScale = Math.min(1.22, Math.max(1, (expandedWidth - 24) / 46));
		const scales = centers.map((center) =>
			pointer === null ? 1 : 1 + (maxScale - 1) * Math.exp(-(((center - pointer) / 72) ** 2))
		);
		const left = 12 + (first.width * scales[0]) / 2;
		const right = expandedWidth - 12 - (last.width * scales[scales.length - 1]) / 2;
		// A smooth, monotonic lens normalized at both ends. No per-item branches or
		// fit/overflow switch: crossing a thumbnail or reaching an end never snaps.
		const warp = (center: number) =>
			pointer === null
				? center
				: center +
					(maxScale - 1) * (center - pointer) * Math.exp(-(((center - pointer) / 72) ** 2));
		const start = warp(centers[0]);
		const end = warp(centers[centers.length - 1]);
		return thumbnails.map((t, i) => ({
			x:
				(thumbnails.length === 1
					? expandedWidth / 2
					: left + ((warp(centers[i]) - start) / (end - start || 1)) * Math.max(0, right - left)) -
				t.width / 2,
			scale: scales[i]
		}));
	}

	let restingPositions = $derived(positions(null));
	let dockPositions = $derived(
		positions(activeIndex === null ? null : Math.min(activeIndex, thumbnails.length - 1))
	);

	function centerAt(layout: { x: number }[], index: number) {
		const lo = Math.floor(index);
		const hi = Math.min(lo + 1, layout.length - 1);
		const start = layout[lo].x + thumbnails[lo].width / 2;
		const end = layout[hi].x + thumbnails[hi].width / 2;
		return start + (end - start) * (index - lo);
	}

	function trackPointer(event: PointerEvent) {
		if (event.pointerType !== 'mouse') return;
		const x = event.clientX - (event.currentTarget as HTMLElement).getBoundingClientRect().left;
		for (let i = 0; i < thumbnails.length - 1; i++) {
			const start = centerAt(restingPositions, i);
			const end = centerAt(restingPositions, i + 1);
			if (x <= end) {
				hoverIndex = i + Math.max(0, (x - start) / (end - start));
				return;
			}
		}
		hoverIndex = thumbnails.length - 1;
	}

	function visit(index: number) {
		previewIndex = null;
		const delta = index - (stack.length - 1);
		if (delta) history.go(delta);
		else onCurrent();
	}
</script>

{#if entries.length}
	<nav
		aria-label="Image trail"
		class="trail"
		class:open
		{@attach measureSpace}
		style:--expanded-width={`${expandedWidth}px`}
		onpointerenter={(event) => {
			if (event.pointerType === 'mouse') {
				hovered = true;
				trackPointer(event);
			}
		}}
		onpointermove={trackPointer}
		onpointerleave={() => {
			hovered = false;
			focused = false;
			hoverIndex = null;
			previewIndex = null;
		}}
		onfocusin={() => (focused = true)}
		onfocusout={(event) => {
			if (!event.currentTarget.contains(event.relatedTarget as Node)) focused = false;
		}}
		transition:fly={{
			y: prefersReducedMotion.current ? 0 : -6,
			duration: prefersReducedMotion.current ? 0 : 220
		}}
	>
		<div class="pictures">
			{#each thumbnails as { save, image, width, height }, i (i)}
				{@const visibility = effectiveVisibility(save.labels ?? [])}
				<HoverCard.Root
					bind:open={
						() => previewIndex === i,
						(value) => {
							if (value && open) previewIndex = i;
							else if (previewIndex === i) previewIndex = null;
						}
					}
					disabled={!image || visibility === 'hide' || !!(save.labels?.length && prefsPending())}
					openDelay={500}
					closeDelay={150}
					ignoreNonKeyboardFocus
				>
					<HoverCard.Trigger
						onfocus={() => (focusIndex = i)}
						onkeydown={(event) => {
							if (event.key === 'Escape') {
								hovered = focused = false;
								previewIndex = null;
								(event.target as HTMLElement).blur();
							}
						}}
						onpointerdown={() => (previewIndex = null)}
						oncontextmenu={() => (previewIndex = null)}
					>
						{#snippet child({ props })}
							<button
								{...props}
								type="button"
								class="picture bg-muted"
								style:--x={`${dockPositions[i].x}px`}
								style:--scale={dockPositions[i].scale}
								style:z-index={open ? Math.round(dockPositions[i].scale * 100) : i}
								style:--width={`${width}px`}
								style:--height={`${height}px`}
								style:--offset={Math.min(entries.length - 1 - i, 3)}
								style:--rotation={`${[-8, 5, -3, 9, -5][i % 5]}deg`}
								aria-label={`Return to image ${i + 1}${image?.alt ? `: ${image.alt}` : ''}`}
								aria-current={i === stack.length - 1 ? 'step' : undefined}
								onclick={() => visit(i)}
								in:fly={{
									y: prefersReducedMotion.current ? 0 : -8,
									duration: prefersReducedMotion.current ? 0 : 240
								}}
							>
								{#if image && visibility !== 'hide' && !(save.labels?.length && prefsPending())}
									<div class="h-full w-full overflow-hidden" class:blur-md={visibility === 'blur'}>
										<SaveImage
											{image}
											alt=""
											variant="grid"
											sizes="48px"
											loading="eager"
											class="h-full w-full object-cover"
											wrapperClass="h-full w-full"
											overlayFit="object-cover"
										/>
									</div>
								{:else}
									<span class="text-xs text-muted-foreground">{i + 1}</span>
								{/if}
							</button>
						{/snippet}
					</HoverCard.Trigger>
					<HoverCard.Content
						side="bottom"
						align="center"
						sideOffset={12}
						class="w-auto max-w-[min(18rem,calc(100vw-2rem))] p-1 data-closed:hidden data-closed:animate-none"
					>
						{#if image && previewIndex === i}
							<div data-trail-preview={save.uri} class="overflow-hidden rounded-lg">
								<LabeledMedia labels={save.labels} class="flex items-center justify-center">
									<SaveImage
										{image}
										alt={image.alt ?? 'Image preview'}
										sizes="280px"
										loading="eager"
										class="max-h-[min(22rem,50vh)] w-auto max-w-full object-contain"
										wrapperClass="flex max-h-[min(22rem,50vh)] max-w-full items-center justify-center"
									/>
								</LabeledMedia>
							</div>
						{/if}
					</HoverCard.Content>
				</HoverCard.Root>
			{/each}
		</div>
	</nav>
{/if}

<style>
	.trail {
		position: relative;
		pointer-events: none;
		flex-shrink: 0;
		width: var(--expanded-width);
		height: 166px;
		overflow: clip;
	}

	.pictures {
		position: absolute;
		top: 60px;
		width: 100%;
		height: 57px;
		pointer-events: none;
		/* Cast one shadow from the combined silhouette, independent of history depth. */
		filter: drop-shadow(0 5px 7px #00000040);
	}
	.open .pictures {
		pointer-events: auto;
	}
	.open .pictures::before {
		content: '';
		position: absolute;
		inset: -6px;
	}
	.picture {
		position: absolute;
		top: 0;
		pointer-events: auto;
		transform-origin: top center;
		left: 0;
		width: var(--width);
		height: var(--height);
		border: 0;
		border-radius: 7px;
		overflow: hidden;
		cursor: pointer;
		transform: translateX(calc(12px + (46px - var(--width)) / 2 + var(--offset) * 3px))
			rotate(var(--rotation));
		transition: transform 380ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.open .picture {
		transform: translateX(var(--x)) rotate(0deg) scale(var(--scale));
		transition-duration: 160ms;
	}
	.picture::after {
		content: '';
		position: absolute;
		inset: 0;
		border: 1px solid rgb(255 255 255 / 0.85);
		border-radius: inherit;
		pointer-events: none;
	}
	.open .picture:focus-visible {
		outline: 2px solid var(--ring);
		outline-offset: 1px;
	}

	@media (prefers-reduced-motion: reduce) {
		.picture {
			transition: none;
		}
	}
</style>
