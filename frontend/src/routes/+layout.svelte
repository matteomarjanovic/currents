<script lang="ts">
	import { onMount } from 'svelte';
	import { initApp } from '$lib/app-init';

	let { children } = $props();

	onMount(() => {
		initApp().catch((err) => console.warn('initApp failed', err));
	});
</script>

{@render children()}
