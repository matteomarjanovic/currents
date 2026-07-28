<script lang="ts">
	import { onMount } from 'svelte';
	import { supporter, loadSupporterStatus } from '$lib/stores/supporter.svelte';

	// The color-search trial counter, shown under the color panel of both search
	// commands so the wording can't drift. The panel is remounted every time it
	// opens, so refreshing here keeps the number honest — the server owns it,
	// and a search from another tab or the phone already spent from the same
	// allowance.
	onMount(() => {
		if (!supporter.active) void loadSupporterStatus();
	});
</script>

{#if supporter.loaded && !supporter.active}
	<p class="text-center text-xs text-muted-foreground">
		{#if supporter.colorTrialsLeft > 0}
			{supporter.colorTrialsLeft} free color {supporter.colorTrialsLeft === 1
				? 'search'
				: 'searches'} left
		{:else}
			You’ve used your free color searches. Supporters search any color, any time.
		{/if}
	</p>
{/if}
