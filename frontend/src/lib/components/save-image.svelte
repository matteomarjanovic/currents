<script lang="ts">
	import type { ImageContentView } from '$lib/types';
	import { preferences } from '$lib/stores/preferences.svelte';
	import { bunnyImageSrcset, bunnyImageUrl, type ImageTransform } from '$lib/image-url';
	import { isCropped } from '$lib/image-ratio';

	interface Props {
		image: ImageContentView;
		alt: string;
		// Classes / inline style applied to the <img> (unchanged from a plain render).
		class?: string;
		style?: string;
		// Classes for the freeze wrapper (used only when a GIF is frozen). Give it the
		// same sizing context the <img> expects so the overlay canvas bounds the image:
		// grid → "block w-full", full-height contain → "flex h-full w-full items-center
		// justify-center", intrinsic → leave empty.
		wrapperClass?: string;
		// object-fit/position classes for the frozen-GIF canvas. Must match the <img>'s
		// own fit, or the still frame won't line up with the animation beneath it —
		// a cropped grid tile covers, everything else contains.
		overlayFit?: string;
		// Lazy by default. Set "eager" for an image that is deliberately parked outside
		// the viewport and has to be ready the moment it moves in — the detail view's
		// swipe neighbours, which lazy loading would otherwise hold back until they're
		// already sliding into view.
		loading?: 'lazy' | 'eager';
		variant?: 'grid' | 'detail';
		sizes?: string;
	}

	let {
		image,
		alt,
		class: className = '',
		style,
		wrapperClass = '',
		overlayFit = 'object-contain',
		loading = 'lazy',
		variant = 'detail',
		sizes
	}: Props = $props();

	const widths = $derived(variant === 'grid' ? [320, 480, 640, 960] : [640, 960, 1440, 2048]);
	const fallbackWidth = $derived(variant === 'grid' ? 640 : 1440);
	const transform = $derived.by((): Omit<ImageTransform, 'width'> => {
		if (variant !== 'grid' || !isCropped(image.width, image.height) || !image.width) {
			return { quality: variant === 'grid' ? 80 : 85 };
		}
		return {
			crop: `${image.width},${image.width * 2}`,
			cropGravity: 'north',
			quality: 80
		};
	});
	const src = $derived(
		bunnyImageUrl(image.imageUrl, {
			...transform,
			width: Math.min(image.width ?? fallbackWidth, fallbackWidth)
		})
	);
	const srcset = $derived(
		image.width
			? bunnyImageSrcset(image.imageUrl, widths, image.width, transform) || undefined
			: undefined
	);
	const imageSizes = $derived(
		sizes ??
			(variant === 'grid'
				? '(max-width: 623px) calc(50vw - 1.5rem), 200px'
				: '(max-width: 767px) calc(100vw - 1rem), 67vw')
	);

	// Freeze animated GIFs at their first frame unless the viewer opted into autoplay.
	// Everything else (and users on the default) takes the plain <img> path — no change.
	let freeze = $derived(image.mimeType === 'image/gif' && !preferences.gifAutoplay);

	let imgEl = $state<HTMLImageElement>();
	let canvasEl = $state<HTMLCanvasElement>();
	let hovering = $state(false);
	let frozen = $state(false);

	// An <img> keeps painting its previous bitmap until the next src finishes loading.
	// Replace the node when a save changes so a detail view never shows the wrong image
	// during that gap. The responsive srcset can still show the smaller rendition first.
	$effect(() => {
		void src;
		frozen = false;
		hovering = false;
	});

	// Paint the GIF's first frame onto the canvas once it decodes. A freshly loaded
	// GIF sits on frame 0, so drawing on `load` captures a static first frame. We only
	// ever draw (never read pixels back), so a cross-origin CDN image doesn't trip the
	// tainted-canvas restriction.
	function drawFirstFrame() {
		if (!freeze || !imgEl || !canvasEl) return;
		const w = imgEl.naturalWidth;
		const h = imgEl.naturalHeight;
		if (!w || !h) return;
		canvasEl.width = w;
		canvasEl.height = h;
		canvasEl.getContext('2d')?.drawImage(imgEl, 0, 0);
		frozen = true;
	}
</script>

{#key src}
	{#if freeze}
		<div
			class="relative {wrapperClass}"
			role="img"
			aria-label={alt}
			onpointerenter={() => (hovering = true)}
			onpointerleave={() => (hovering = false)}
		>
			<img
				bind:this={imgEl}
				{src}
				{srcset}
				sizes={imageSizes}
				alt=""
				{loading}
				class={className}
				{style}
				onload={drawFirstFrame}
			/>
			<!-- Frozen first frame overlays the animating <img>; hidden on hover to reveal playback. -->
			<canvas
				bind:this={canvasEl}
				aria-hidden="true"
				class="pointer-events-none absolute inset-0 h-full w-full {overlayFit} transition-opacity duration-150 {frozen &&
				!hovering
					? 'opacity-100'
					: 'opacity-0'}"
			></canvas>
		</div>
	{:else}
		<img {src} {srcset} sizes={imageSizes} {alt} {loading} class={className} {style} />
	{/if}
{/key}
