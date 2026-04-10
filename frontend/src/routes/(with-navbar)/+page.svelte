<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import LandingPage from '$lib/components/landing-page.svelte';

	$effect(() => {
		if (auth.checked && auth.user) goto('/explore');
	});

	onMount(() => {
		const wasDark = document.documentElement.classList.contains('dark');
		document.documentElement.classList.add('dark');
		return () => {
			if (!wasDark) document.documentElement.classList.remove('dark');
		};
	});
</script>

<LandingPage />
