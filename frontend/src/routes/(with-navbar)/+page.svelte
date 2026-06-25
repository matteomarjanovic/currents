<script lang="ts">
	import { onMount } from 'svelte';
	import LandingPage from '$lib/components/landing-page.svelte';
	import MobileWelcome from '$lib/components/mobile-welcome.svelte';
	import { isNative } from '$lib/platform';

	const native = isNative();

	onMount(() => {
		// The native welcome screen brings its own dark video backdrop and respects the
		// user's theme, so only the web landing page forces dark mode.
		if (native) return;
		const wasDark = document.documentElement.classList.contains('dark');
		document.documentElement.classList.add('dark');
		return () => {
			if (!wasDark) document.documentElement.classList.remove('dark');
		};
	});
</script>

{#if native}
	<MobileWelcome />
{:else}
	<LandingPage />
{/if}
