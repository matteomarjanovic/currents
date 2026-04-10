<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { setMode, resetMode } from 'mode-watcher';
	import { auth } from '$lib/stores/auth.svelte';
	import LandingPage from '$lib/components/landing-page.svelte';

	$effect(() => {
		if (auth.checked && auth.user) goto('/explore');
	});

	onMount(() => {
		setMode('dark');
		return () => resetMode();
	});
</script>

<LandingPage />
