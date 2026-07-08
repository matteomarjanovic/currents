<script lang="ts">
	import LogoIcon from '$lib/assets/logo-icon.svelte';
	import { cn } from '$lib/utils.js';

	// Supporter badge: a "negative" heart — the Currents wave mark carved inside
	// a heart outline, with the wave curves and the heart border painted pink and
	// the interior transparent. Sized like an icon via width/height classes
	// (defaults to size-4); pink in both themes and independent of the
	// surrounding text color (pinned via fill/stroke utilities).
	let { class: className = '' }: { class?: string } = $props();

	// Material Symbols "favorite" heart geometry (Apache 2.0), 960-unit grid.
	const HEART =
		'm480-120-58-52q-101-91-167-157T150-447.5Q111-500 95.5-544T80-634q0-94 63-157t157-63q52 0 99 22t81 62q34-40 81-62t99-22q94 0 157 63t63 157q0 46-15.5 90T810-447.5Q771-395 705-329T538-172l-58 52Z';
	const uid = $props.id();
</script>

<span
	class={cn('inline-block size-4 shrink-0 align-middle', className)}
	role="img"
	aria-label="Currents supporter badge"
	title="Currents supporter"
>
	<svg viewBox="0 -960 960 960" class="h-full w-full overflow-visible" aria-hidden="true">
		<defs>
			<!-- White wave marks the pink stripes; everything else stays masked out.
			     The wave is oversized to 135% of the box (centered on the heart's
			     optical center) so bolder segments carve the shape. -->
			<mask id="supporter-wave-{uid}">
				<svg x="-155" y="-1147" width="1270" height="1296" style="color: white">
					<LogoIcon />
				</svg>
			</mask>
		</defs>
		<!-- Pink wave stripes, bounded to the heart by painting the heart shape. -->
		<path d={HEART} class="fill-pink-500" mask="url(#supporter-wave-{uid})" />
		<!-- Pink heart outline so the silhouette reads at username sizes. -->
		<path d={HEART} fill="none" class="stroke-pink-500" stroke-width="40" />
	</svg>
</span>
