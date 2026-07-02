<script lang="ts">
	import LogoIcon from '$lib/assets/logo-icon.svelte';
	import { cn } from '$lib/utils.js';

	// Supporter badge: the Currents mark inscribed in a heart. Sized like an
	// icon via width/height classes (defaults to size-4). Colors are fixed —
	// pink heart, white mark — so it reads the same in both themes.
	let { class: className = '' }: { class?: string } = $props();
</script>

<span
	class={cn('relative inline-block size-4 shrink-0 align-middle', className)}
	role="img"
	aria-label="Currents supporter badge"
	title="Currents supporter"
>
	<svg
		viewBox="0 0 24 24"
		class="absolute inset-0 h-full w-full text-pink-500"
		fill="currentColor"
		stroke="currentColor"
		stroke-width="2"
		stroke-linejoin="round"
		aria-hidden="true"
	>
		<path
			d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z"
		/>
	</svg>
	<!-- 54% box whose center sits at (50%, 42%) — the heart's optical center. -->
	<span
		class="absolute flex items-center justify-center text-white"
		style="left: 23%; top: 15%; width: 54%; height: 54%;"
	>
		<LogoIcon />
	</span>
</span>
