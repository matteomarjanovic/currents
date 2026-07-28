<script lang="ts">
	// The Lucide "search" icon, optionally with its lens filled by the color being
	// searched — the top bar wears the query's color while a color (or hybrid)
	// search is active. Geometry matches @lucide/svelte's search icon; the fill
	// radius sits inside the ring's inner edge (r 8 minus the 2-wide stroke).
	let { color, class: className }: { color?: string; class?: string } = $props();
</script>

<svg
	xmlns="http://www.w3.org/2000/svg"
	viewBox="0 0 24 24"
	width="24"
	height="24"
	fill="none"
	stroke="currentColor"
	stroke-width="2"
	stroke-linecap="round"
	stroke-linejoin="round"
	class={className}
	aria-hidden="true"
>
	{#if color}
		<circle cx="11" cy="11" r="5" fill={color} stroke="none" />
	{/if}
	<path d="m21 21-4.34-4.34" />
	<circle cx="11" cy="11" r="8" />
</svg>
