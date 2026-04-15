<script lang="ts">
	import { onMount } from 'svelte';
	import LandingPage from '$lib/components/landing-page.svelte';

	onMount(() => {
		const wasDark = document.documentElement.classList.contains('dark');
		document.documentElement.classList.add('dark');
		return () => {
			if (!wasDark) document.documentElement.classList.remove('dark');
		};
	});
</script>

<LandingPage />
