<script lang="ts">
	import LogoIcon from '$lib/assets/logo-icon.svelte';
	import { cn } from '$lib/utils.js';

	// Supporter badge: the Currents wave curves clipped by a heart shape, with
	// the clipping heart traced as a thin outline so the silhouette stays
	// legible at username sizes. Sized like an icon via width/height classes
	// (defaults to size-4); pink in both themes.
	let { class: className = '' }: { class?: string } = $props();

	// Material Symbols "favorite" heart geometry (Apache 2.0), 960-unit grid.
	const HEART =
		'm480-120-58-52q-101-91-167-157T150-447.5Q111-500 95.5-544T80-634q0-94 63-157t157-63q52 0 99 22t81 62q34-40 81-62t99-22q94 0 157 63t63 157q0 46-15.5 90T810-447.5Q771-395 705-329T538-172l-58 52Z';
	const uid = $props.id();
</script>

<span
	class={cn('relative inline-block size-4 shrink-0 align-middle text-pink-500', className)}
	role="img"
	aria-label="Currents supporter badge"
	title="Currents supporter"
>
	<svg width="0" height="0" class="absolute" aria-hidden="true">
		<defs>
			<clipPath id="supporter-heart-{uid}" clipPathUnits="objectBoundingBox">
				<path transform="scale(0.00104167) translate(0 960)" d={HEART} />
			</clipPath>
		</defs>
	</svg>
	<!-- The wave mark oversized to 135% of the box (centered on the heart's
	     optical center) so bolder curve segments fill the clipped area. -->
	<span class="absolute inset-0" style="clip-path: url(#supporter-heart-{uid})">
		<span
			class="absolute flex items-center justify-center"
			style="left: -17.5%; top: -19.5%; width: 135%; height: 135%;"
		>
			<LogoIcon />
		</span>
	</span>
	<svg
		viewBox="0 -960 960 960"
		class="absolute inset-0 h-full w-full overflow-visible"
		fill="none"
		stroke="currentColor"
		stroke-width="40"
		aria-hidden="true"
	>
		<path d={HEART} />
	</svg>
</span>
