<script lang="ts">
	import { onDestroy, type Snippet } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { SvelteMap } from 'svelte/reactivity';
	import { swipe } from '$lib/swipe';
	import { getImageContent, type SaveView } from '$lib/types';
	import { isNative } from '$lib/platform';
	import SaveImage from '$lib/components/save-image.svelte';
	import LabeledMedia from '$lib/components/labeled-media.svelte';

	let {
		save,
		previous,
		next,
		enabled,
		onNavigate,
		onContinuePinch,
		children
	}: {
		save: SaveView;
		previous: SaveView | null;
		next: SaveView | null;
		enabled: boolean;
		onNavigate: (direction: -1 | 1) => void;
		onContinuePinch: (pointers: { id: number; x: number; y: number }[]) => void;
		children: Snippet;
	} = $props();

	const native = isNative();
	let width = $state(0);
	let x = $state(0);
	let eased = $state(false);
	let swiping = $state(false);
	let disposed = false;
	const pointers = new SvelteMap<number, { id: number; x: number; y: number }>();
	let step = $derived(width + 24);
	let neighbours = $derived(
		enabled
			? [
					{ save: previous, direction: -1 },
					{ save: next, direction: 1 }
				].filter((entry): entry is { save: SaveView; direction: number } => !!entry.save)
			: []
	);

	$effect(() => {
		void save.uri;
		pointers.clear();
		x = 0;
		eased = false;
	});
	onDestroy(() => {
		disposed = true;
	});

	function move(dx: number) {
		if (swiping) return;
		eased = false;
		x = (dx < 0 ? next : previous) ? dx : dx * 0.2;
	}
	function cancel() {
		if (swiping) return;
		eased = true;
		x = 0;
	}
	async function navigate(direction: -1 | 1) {
		if (swiping || !(direction === 1 ? next : previous)) return cancel();
		const uri = save.uri;
		swiping = true;
		eased = true;
		x = -direction * step;
		if (!prefersReducedMotion.current) await new Promise((resolve) => setTimeout(resolve, 200));
		if (!disposed && enabled && save.uri === uri) onNavigate(direction);
		eased = false;
		x = 0;
		swiping = false;
	}

	function pointerDown(event: PointerEvent) {
		// Only a pinch on the image itself can enter focus; moderation overlays stay in control.
		if (
			!native ||
			event.pointerType !== 'touch' ||
			!(event.target as HTMLElement).closest('[data-swipe-image]')
		)
			return;
		pointers.set(event.pointerId, { id: event.pointerId, x: event.clientX, y: event.clientY });
		if (pointers.size !== 2) return;
		const node = event.currentTarget as HTMLElement;
		for (const id of pointers.keys()) node.setPointerCapture(id);
		onContinuePinch([...pointers.values()]);
	}
	function pointerMove(event: PointerEvent) {
		if (pointers.has(event.pointerId))
			pointers.set(event.pointerId, { id: event.pointerId, x: event.clientX, y: event.clientY });
	}
</script>

<div
	data-organize-swipe-stage
	data-swiping={swiping}
	class="relative flex w-full shrink-0 justify-center overflow-hidden"
	bind:clientWidth={width}
	use:swipe={{
		enabled: enabled && !!(previous || next),
		onMove: move,
		onSwipe: navigate,
		onCancel: cancel
	}}
>
	<div
		data-swipe-current
		role="group"
		aria-label="Selected image"
		class="flex w-full justify-center {eased && !prefersReducedMotion.current
			? 'transition-transform duration-200 ease-out'
			: ''}"
		style:transform={`translate3d(${x}px, 0, 0)`}
		style:touch-action={native ? 'pan-y' : undefined}
		onpointerdown={pointerDown}
		onpointermove={pointerMove}
		onpointerup={(event) => pointers.delete(event.pointerId)}
		onpointercancel={(event) => pointers.delete(event.pointerId)}
	>
		{@render children()}
	</div>
	{#each neighbours as neighbour (neighbour.direction)}
		{@const image = getImageContent(neighbour.save)}
		<div
			data-swipe-neighbour={neighbour.direction}
			inert
			aria-hidden="true"
			class="pointer-events-none absolute inset-y-0 left-0 flex w-full items-center justify-center {eased &&
			!prefersReducedMotion.current
				? 'transition-transform duration-200 ease-out'
				: ''}"
			style:transform={`translate3d(${x + neighbour.direction * step}px, 0, 0)`}
		>
			{#if image}
				<LabeledMedia
					labels={neighbour.save.labels}
					class="flex h-full w-full items-center justify-center"
				>
					<SaveImage
						{image}
						alt=""
						sizes="22rem"
						loading="eager"
						class="h-full w-auto max-w-full object-contain"
						wrapperClass="flex h-full w-full items-center justify-center"
					/>
				</LabeledMedia>
			{/if}
		</div>
	{/each}
</div>
