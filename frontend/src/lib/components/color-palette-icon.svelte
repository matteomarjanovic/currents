<script lang="ts">
	// The Lucide "palette" icon, but with its paint dots individually colored and
	// an optional body fill (used as the color-search toggle: outline when off;
	// when on, the body fills with the muted border color for a solid look — the
	// dots keep their colors). Geometry matches @lucide/svelte's palette icon;
	// dot radius is bumped from 0.5 so the colors are legible at icon sizes.
	let { filled = false, class: className }: { filled?: boolean; class?: string } = $props();

	const DOTS = [
		{ cx: 8.5, cy: 7.5, fill: '#e63946' },
		{ cx: 13.5, cy: 6.5, fill: '#e9c46a' },
		{ cx: 17.5, cy: 10.5, fill: '#2a9d8f' },
		{ cx: 6.5, cy: 12.5, fill: '#457b9d' }
	];
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
	<path
		d="M12 22a1 1 0 0 1 0-20 10 9 0 0 1 10 9 5 5 0 0 1-5 5h-2.25a1.75 1.75 0 0 0-1.4 2.8l.3.4a1.75 1.75 0 0 1-1.4 2.8z"
		fill="none"
		class={filled ? 'fill-muted-foreground stroke-muted-foreground' : ''}
	/>
	{#each DOTS as d (d.fill)}
		<circle cx={d.cx} cy={d.cy} r="1.4" fill={d.fill} stroke="none" />
	{/each}
</svg>
