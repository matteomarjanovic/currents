<script lang="ts">
	import type { ImageContentView } from '$lib/types';
	import { preferences } from '$lib/stores/preferences.svelte';

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
	}

	let {
		image,
		alt,
		class: className = '',
		style,
		wrapperClass = '',
		overlayFit = 'object-contain'
	}: Props = $props();

	// Freeze animated GIFs at their first frame unless the viewer opted into autoplay.
	// Everything else (and users on the default) takes the plain <img> path — no change.
	let freeze = $derived(image.mimeType === 'image/gif' && !preferences.gifAutoplay);

	let imgEl = $state<HTMLImageElement>();
	let canvasEl = $state<HTMLCanvasElement>();
	let hovering = $state(false);
	let frozen = $state(false);

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
			src={image.imageUrl}
			alt=""
			loading="lazy"
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
	<img src={image.imageUrl} {alt} loading="lazy" class={className} {style} />
{/if}
