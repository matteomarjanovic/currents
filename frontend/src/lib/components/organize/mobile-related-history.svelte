<script lang="ts">
	import { getImageContent, type SaveView } from '$lib/types';
	import { isCropped } from '$lib/image-ratio';
	import { effectiveVisibility, prefsPending } from '$lib/stores/moderation-prefs.svelte';
	import { Dialog } from 'bits-ui';
	import { prefersReducedMotion } from 'svelte/motion';
	import { onBackButton } from '$lib/back-button';
	import { Button } from '$lib/components/ui/button';
	import SaveImage from '$lib/components/save-image.svelte';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import EyeOff from '@lucide/svelte/icons/eye-off';
	import X from '@lucide/svelte/icons/x';

	let {
		saves,
		currentIndex,
		canGoForward = false,
		onVisit,
		onRevisit,
		aboveScrollToTop = false
	}: {
		saves: SaveView[];
		currentIndex: number;
		canGoForward?: boolean;
		onVisit?: (index: number) => void;
		onRevisit: (index: number) => void;
		aboveScrollToTop?: boolean;
	} = $props();
	const STEP = 108;
	const CONTROL_HEIGHT = 58;
	const STACK_WIDTH = 64;
	let thumbnails = $derived(
		saves.map((save, index) => {
			const image = getImageContent(save);
			const ratio = image?.width && image.height ? image.width / image.height : 1;
			const width = Math.min(72, 88 * ratio);
			const height = width / ratio;
			const rotation = [-8, 6, -2][index % 3];
			const offset = Math.min(2, Math.max(0, currentIndex - index)) * 4;
			const radians = (Math.abs(rotation) * Math.PI) / 180;
			const sin = Math.sin(radians);
			const cos = Math.cos(radians);
			// Fit the rotated picture inside the stack hit area without cropping.
			const scale = Math.min(
				CONTROL_HEIGHT / (height * cos + width * sin),
				(STACK_WIDTH - 2 * offset) / (width * cos + height * sin)
			);
			return { save, width, height, rotation, offset, scale };
		})
	);
	let open = $state(false);
	let transitioning = $state(false);
	let list = $state<HTMLDivElement>();
	let scrollTop = $state(0);
	let height = $state(0);
	let overflow = $derived(Math.max(0, (saves.length - 1) * STEP + 132 - height));
	$effect(() => {
		if (!open) return;
		void currentIndex;
		return onBackButton(() => {});
	});

	function closeHistory() {
		transitioning = !prefersReducedMotion.current;
		open = false;
	}
</script>

{#snippet thumbnail(save: SaveView, sizes: string)}
	{@const image = getImageContent(save)}
	{@const visibility = effectiveVisibility(save.labels ?? [])}
	{#if image && visibility !== 'hide' && !(save.labels?.length && prefsPending())}
		<div class="h-full w-full overflow-hidden" class:blur-md={visibility === 'blur'}>
			<SaveImage
				{image}
				alt=""
				variant={isCropped(image.width, image.height) ? 'detail' : 'grid'}
				{sizes}
				loading="eager"
				class="h-full w-full object-contain"
				wrapperClass="h-full w-full"
				overlayFit="object-contain"
			/>
		</div>
	{:else}
		<div class="flex h-full w-full items-center justify-center">
			<EyeOff class="size-5 text-muted-foreground" />
		</div>
	{/if}
{/snippet}

{#snippet navigation()}
	<div
		class="pointer-events-auto flex h-(--control-height) items-center gap-1 rounded-full border border-border bg-background/95 p-1 shadow-lg backdrop-blur-xl"
		role="group"
		aria-label="Related navigation"
	>
		<Button
			variant="ghost"
			class="size-12 rounded-full"
			disabled={currentIndex === 0}
			onclick={() => onVisit?.(currentIndex - 1)}
			aria-label="Back in related images"><ArrowLeft class="size-5" /></Button
		>
		<Button
			variant="ghost"
			class="size-12 rounded-full"
			disabled={!canGoForward}
			onclick={() => onVisit?.(currentIndex + 1)}
			aria-label="Forward in related images"><ArrowRight class="size-5" /></Button
		>
	</div>
{/snippet}

<!-- Keep the journey's native Back handler below this overlay. -->
<Dialog.Root
	bind:open
	onOpenChange={(next) => {
		transitioning = !prefersReducedMotion.current;
		if (next && list) {
			list.scrollTop = overflow;
			scrollTop = list.scrollTop;
		}
	}}
>
	<div
		data-mobile-related-navigation
		class="history-controls pointer-events-none left-3 z-30 flex items-center justify-between gap-3 {aboveScrollToTop
			? 'fixed'
			: 'absolute'}"
		class:above-scroll-to-top={aboveScrollToTop}
		inert={open}
		style:--control-height={`${CONTROL_HEIGHT}px`}
		style:--stack-width={`${STACK_WIDTH}px`}
		style:bottom="var(--control-bottom)"
	>
		{#if onVisit}{@render navigation()}{/if}
		<Dialog.Trigger
			aria-label="Open image history"
			class="pointer-events-auto ml-auto h-(--control-height) w-(--stack-width) rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring"
		/>
	</div>
	<Dialog.Portal>
		<Dialog.Overlay forceMount>
			{#snippet child({ props })}
				<div
					{...props}
					data-mobile-history-backdrop={open ? '' : undefined}
					class="history-backdrop fixed inset-0 z-60 bg-black/50"
					style:opacity={open ? 1 : 0}
					style:pointer-events={open ? 'auto' : 'none'}
				></div>
			{/snippet}
		</Dialog.Overlay>
		<Dialog.Content
			forceMount
			onEscapeKeydown={(event) => event.preventDefault()}
			onInteractOutside={(event) => event.preventDefault()}
			onOpenAutoFocus={(event) => {
				event.preventDefault();
				list?.closest<HTMLElement>('[role="dialog"]')?.focus({ preventScroll: true });
			}}
		>
			{#snippet child({ props })}
				<div
					{...props}
					bind:clientHeight={height}
					data-mobile-image-history={open ? '' : undefined}
					data-mobile-history-layer
					role={open ? 'dialog' : undefined}
					tabindex="-1"
					aria-modal={open ? 'true' : undefined}
					aria-hidden={!open}
					inert={!open}
					class="history-layer fixed inset-0 z-60 outline-none"
					class:open
					class:transitioning
					class:above-scroll-to-top={aboveScrollToTop}
					style:--control-height={`${CONTROL_HEIGHT}px`}
					style:--stack-width={`${STACK_WIDTH}px`}
					style:pointer-events={open ? 'auto' : 'none'}
				>
					<Dialog.Title class="sr-only">Image history</Dialog.Title>
					<Dialog.Description class="sr-only"
						>Scroll through the thumbnails to revisit an image. Tap the background or X to close.</Dialog.Description
					>
					<div
						bind:this={list}
						onclickcapture={(event) => {
							if (!(event.target as Element).closest('button')) {
								event.stopPropagation();
								closeHistory();
							}
						}}
						ondragstart={(event) => event.preventDefault()}
						ontouchmove={() => (transitioning = false)}
						onwheel={() => (transitioning = false)}
						onscroll={() => (scrollTop = list!.scrollTop)}
						data-mobile-history-list
						role="group"
						aria-label="Image history"
						class="history-scroll absolute inset-0 touch-pan-y overflow-x-hidden overflow-y-auto overscroll-contain select-none"
					>
						<div style:height={`calc(100% + ${overflow}px)`}>
							<div class="sticky top-0 h-dvh w-full overflow-clip">
								{#each thumbnails as { save, width: thumbnailWidth, height: thumbnailHeight, rotation, offset, scale }, i (i)}
									<button
										type="button"
										aria-label={`Return to image ${i + 1}`}
										aria-current={i === currentIndex ? 'step' : undefined}
										onclick={() => {
											onRevisit(i);
											closeHistory();
										}}
										ontransitionend={(event) => {
											if (event.propertyName === 'transform') transitioning = false;
										}}
										style:width={`${thumbnailWidth}px`}
										style:height={`${thumbnailHeight}px`}
										style:--half-width={`${thumbnailWidth / 2}px`}
										style:--half-height={`${thumbnailHeight / 2}px`}
										style:--offset-y={`${(i - saves.length + 1) * STEP + overflow - scrollTop}px`}
										style:--offset={`${offset}px`}
										style:--rotation={`${rotation}deg`}
										style:--stack-scale={scale}
										style:opacity={open || (i <= currentIndex && i >= currentIndex - 2) ? 1 : 0}
										style:z-index={i === currentIndex ? saves.length : i}
										class="picture absolute top-0 left-0 overflow-hidden rounded-md border border-background bg-muted shadow-lg outline-none focus-visible:ring-2 focus-visible:ring-ring"
									>
										{@render thumbnail(save, '96px')}
									</button>
								{/each}
							</div>
						</div>
					</div>
					<div
						class="pointer-events-none absolute right-24 flex justify-end"
						style:bottom="var(--close-bottom, var(--control-bottom))"
						class:invisible={!open}
					>
						<Dialog.Close
							data-history-close
							aria-label="Close image history"
							class="history-close pointer-events-auto flex size-(--control-height) items-center justify-center rounded-full border border-border bg-background text-foreground shadow-lg outline-none focus-visible:ring-2 focus-visible:ring-ring"
						>
							<X class="size-7" />
						</Dialog.Close>
					</div>
				</div>
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>

<style>
	.history-controls,
	.history-layer {
		--stack-right: 0.75rem;
		--control-bottom: calc(env(safe-area-inset-bottom) + 0.75rem);
	}
	.history-controls {
		right: var(--stack-right);
	}
	.above-scroll-to-top {
		--stack-right: 0.5rem;
		--control-bottom: calc(env(safe-area-inset-bottom) + 4.25rem);
		--close-bottom: calc(env(safe-area-inset-bottom) + 1rem);
	}
	.history-layer {
		--stack-x: calc(100vw - var(--stack-right) - var(--stack-width) / 2);
		--stack-y: calc(100dvh - var(--control-bottom) - var(--control-height) / 2);
	}
	.history-scroll {
		scrollbar-width: none;
	}
	.history-scroll::-webkit-scrollbar {
		display: none;
	}
	.picture {
		transform: translate(
				calc(var(--stack-x) - var(--half-width) + var(--offset)),
				calc(var(--stack-y) - var(--half-height))
			)
			rotate(var(--rotation)) scale(var(--stack-scale));
	}
	.transitioning .picture {
		transition: transform 380ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.open .picture {
		transform: translate(
				calc(100vw - 48px - var(--half-width)),
				calc(100dvh - env(safe-area-inset-bottom) - 56px - var(--half-height) + var(--offset-y))
			)
			rotate(0deg) scale(1);
	}
	.open.transitioning .picture {
		transition-duration: 160ms;
	}

	.history-backdrop {
		transition: opacity 160ms ease-out;
	}
	.history-layer :global(.history-close) {
		opacity: 0;
		transition: opacity 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.history-layer.open :global(.history-close) {
		opacity: 1;
	}
	@media (prefers-reduced-motion: reduce) {
		.history-layer .picture,
		.history-backdrop,
		.history-layer :global(.history-close) {
			transition: none;
		}
	}
</style>
