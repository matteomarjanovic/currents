<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import SaveImage from '$lib/components/save-image.svelte';
	import X from '@lucide/svelte/icons/x';
	import { SvelteMap } from 'svelte/reactivity';
	import { onBackButton } from '$lib/back-button';
	import type { ImageContentView } from '$lib/types';

	interface Props {
		open: boolean;
		image: ImageContentView;
		alt: string;
	}

	let { open = $bindable(), image, alt }: Props = $props();

	const MIN_SCALE = 1;
	const MAX_SCALE = 5;
	const DOUBLE_TAP_SCALE = 2.5;
	const WHEEL_SENSITIVITY = 0.0015;

	let viewport: HTMLDivElement | undefined = $state();
	let zoomContent: HTMLDivElement | undefined = $state();
	let scale = $state(MIN_SCALE);
	let x = $state(0);
	let y = $state(0);
	let moving = $state(false);

	type Point = { x: number; y: number };
	const pointers = new SvelteMap<number, Point>();
	let startDistance = 0;
	let startScale = MIN_SCALE;
	let startMidpoint: Point = { x: 0, y: 0 };
	let startX = 0;
	let startY = 0;
	let panStart: Point = { x: 0, y: 0 };
	let backdropPointer: (Point & { id: number }) | undefined;

	function reset() {
		pointers.clear();
		backdropPointer = undefined;
		startDistance = 0;
		scale = MIN_SCALE;
		x = 0;
		y = 0;
		moving = false;
	}

	$effect(() => {
		if (!open) reset();
	});

	$effect(() => {
		if (!open) return;
		return onBackButton(() => (open = false));
	});

	$effect(() => {
		if (open && viewport && pointers.size >= 2 && !startDistance) startPinch();
	});

	// A native pinch begins on the image underneath this dialog. Those active pointers
	// keep targeting that image after the dialog opens, so seed them here and consume the
	// rest of their moves from <svelte:window> below.
	export function continuePinch(initialPointers: { id: number; x: number; y: number }[]) {
		reset();
		for (const point of initialPointers) pointers.set(point.id, point);
		moving = true;
		open = true;
	}

	function distance(a: Point, b: Point) {
		return Math.hypot(a.x - b.x, a.y - b.y);
	}

	function midpoint(a: Point, b: Point): Point {
		return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
	}

	function fromCentre(point: Point): Point {
		if (!viewport) return point;
		const rect = viewport.getBoundingClientRect();
		return {
			x: point.x - rect.left - rect.width / 2,
			y: point.y - rect.top - rect.height / 2
		};
	}

	function clamped(nextScale: number, nextX: number, nextY: number) {
		const img = zoomContent?.querySelector('img');
		if (!viewport || !img) return { x: nextX, y: nextY };
		const maxX = Math.max(0, (img.offsetWidth * nextScale - viewport.clientWidth) / 2);
		const maxY = Math.max(0, (img.offsetHeight * nextScale - viewport.clientHeight) / 2);
		return {
			x: Math.max(-maxX, Math.min(maxX, nextX)),
			y: Math.max(-maxY, Math.min(maxY, nextY))
		};
	}

	function setTransform(nextScale: number, nextX: number, nextY: number) {
		scale = Math.max(MIN_SCALE, Math.min(MAX_SCALE, nextScale));
		const position = clamped(scale, nextX, nextY);
		x = position.x;
		y = position.y;
	}

	function startPinch() {
		const [a, b] = [...pointers.values()];
		startDistance = distance(a, b);
		startScale = scale;
		startMidpoint = fromCentre(midpoint(a, b));
		startX = x;
		startY = y;
	}

	function onPointerDown(e: PointerEvent) {
		if (e.pointerType === 'mouse' && e.button !== 0) return;
		if (backdropPointer && backdropPointer.id !== e.pointerId) {
			backdropPointer = undefined;
			return;
		}
		if (!zoomContent?.contains(e.target as Node)) {
			if (pointers.size > 0) return;
			backdropPointer = { id: e.pointerId, x: e.clientX, y: e.clientY };
			return;
		}
		viewport?.setPointerCapture(e.pointerId);
		pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
		moving = true;
		if (pointers.size === 2) {
			startPinch();
		} else if (pointers.size === 1) {
			panStart = { x: e.clientX, y: e.clientY };
			startX = x;
			startY = y;
		}
	}

	function onPointerMove(e: PointerEvent) {
		if (backdropPointer?.id === e.pointerId) {
			if (distance(backdropPointer, { x: e.clientX, y: e.clientY }) > 8) {
				backdropPointer = undefined;
			}
			return;
		}
		if (!open || !pointers.has(e.pointerId)) return;
		pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });

		if (pointers.size >= 2) {
			if (!startDistance) return startPinch();
			const [a, b] = [...pointers.values()];
			const nextScale = startScale * (distance(a, b) / startDistance);
			const nextMidpoint = fromCentre(midpoint(a, b));
			const ratio = Math.max(MIN_SCALE, Math.min(MAX_SCALE, nextScale)) / startScale;
			setTransform(
				nextScale,
				nextMidpoint.x - (startMidpoint.x - startX) * ratio,
				nextMidpoint.y - (startMidpoint.y - startY) * ratio
			);
		} else if (scale > MIN_SCALE) {
			setTransform(scale, startX + e.clientX - panStart.x, startY + e.clientY - panStart.y);
		}
	}

	function onPointerEnd(e: PointerEvent) {
		if (backdropPointer?.id === e.pointerId) {
			backdropPointer = undefined;
			open = false;
			return;
		}
		if (!open || !pointers.delete(e.pointerId)) return;
		if (pointers.size === 1) {
			const [point] = pointers.values();
			panStart = point;
			startX = x;
			startY = y;
		} else if (pointers.size === 0) {
			moving = false;
			setTransform(scale, x, y);
		}
	}

	function onPointerCancel(e: PointerEvent) {
		if (backdropPointer?.id === e.pointerId) backdropPointer = undefined;
		onPointerEnd(e);
	}

	function toggleZoom(e: MouseEvent) {
		if (window.matchMedia('(pointer: fine)').matches) return;
		e.stopPropagation();
		if (scale > MIN_SCALE) return reset();
		const point = fromCentre({ x: e.clientX, y: e.clientY });
		setTransform(
			DOUBLE_TAP_SCALE,
			point.x - point.x * DOUBLE_TAP_SCALE,
			point.y - point.y * DOUBLE_TAP_SCALE
		);
	}

	function onWheel(e: WheelEvent) {
		if (!window.matchMedia('(pointer: fine)').matches) return;
		e.preventDefault();
		const unit =
			e.deltaMode === WheelEvent.DOM_DELTA_LINE
				? 16
				: e.deltaMode === WheelEvent.DOM_DELTA_PAGE
					? (viewport?.clientHeight ?? 800)
					: 1;
		const nextScale = Math.max(
			MIN_SCALE,
			Math.min(MAX_SCALE, scale * Math.exp(-e.deltaY * unit * WHEEL_SENSITIVITY))
		);
		if (nextScale === scale) return;
		const point = fromCentre({ x: e.clientX, y: e.clientY });
		const ratio = nextScale / scale;
		setTransform(nextScale, point.x - (point.x - x) * ratio, point.y - (point.y - y) * ratio);
	}
</script>

<svelte:window
	onpointermove={onPointerMove}
	onpointerup={onPointerEnd}
	onpointercancel={onPointerCancel}
/>

<Dialog.Root bind:open>
	<Dialog.Content
		showCloseButton={false}
		class="fixed inset-0 top-0 left-0 z-[60] block h-dvh max-h-none! w-screen max-w-none! translate-x-0 translate-y-0 overflow-hidden rounded-none bg-black/85 p-0 text-white shadow-none ring-0"
	>
		<Dialog.Title class="sr-only">Image viewer</Dialog.Title>
		<Dialog.Description class="sr-only">
			Pinch or scroll to zoom, drag to move the image, or double-tap to zoom in and out.
		</Dialog.Description>

		<div
			bind:this={viewport}
			data-image-focus
			role="group"
			aria-label="Zoomable image"
			class="absolute inset-0 flex touch-none items-center justify-center overflow-hidden p-4 select-none md:flex-col md:gap-3 {scale >
			MIN_SCALE
				? moving
					? 'cursor-grabbing'
					: 'cursor-grab'
				: 'cursor-default'}"
			onpointerdown={onPointerDown}
			onwheel={onWheel}
		>
			<div
				bind:this={zoomContent}
				role="presentation"
				class="flex items-center justify-center {moving
					? ''
					: 'transition-transform duration-100 ease-out'}"
				style="transform: translate3d({x}px, {y}px, 0) scale({scale})"
				ondblclick={toggleZoom}
				ondragstart={(e) => e.preventDefault()}
			>
				<SaveImage
					{image}
					{alt}
					loading="eager"
					class="max-h-[calc(100dvh-2rem)] max-w-[calc(100vw-2rem)] object-contain md:max-h-[calc(100dvh-5rem)]"
					wrapperClass="flex items-center justify-center"
					sizes="100vw"
				/>
			</div>
			<div
				aria-hidden={scale > MIN_SCALE}
				class="pointer-events-none hidden shrink-0 rounded-full bg-black/60 px-4 py-2 text-sm text-white/90 backdrop-blur-sm transition-opacity md:block {scale ===
				MIN_SCALE
					? 'opacity-100'
					: 'opacity-0'}"
			>
				Scroll to zoom
			</div>
		</div>

		<Dialog.Close
			class="absolute right-4 z-10 flex size-11 items-center justify-center rounded-full bg-black/50 text-white backdrop-blur-sm"
			style="top: calc(env(safe-area-inset-top) + 1rem)"
			aria-label="Close image viewer"
		>
			<X class="size-5" />
		</Dialog.Close>
		{#if scale === MIN_SCALE}
			<div
				class="pointer-events-none absolute bottom-[calc(env(safe-area-inset-bottom)+1rem)] left-1/2 -translate-x-1/2 rounded-full bg-black/50 px-3 py-1.5 text-xs text-white/90 backdrop-blur-sm md:hidden"
			>
				Pinch to zoom
			</div>
		{/if}
	</Dialog.Content>
</Dialog.Root>
