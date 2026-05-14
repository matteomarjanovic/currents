<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import LandingPage from '$lib/components/landing-page.svelte';
	import MobileWelcome from '$lib/components/mobile-welcome.svelte';
	import { isNative } from '$lib/platform';

	const native = isNative();

	onMount(() => {
		if (native) return;
		const wasDark = document.documentElement.classList.contains('dark');
		document.documentElement.classList.add('dark');
		return () => {
			if (!wasDark) document.documentElement.classList.remove('dark');
		};
	});
</script>

<svelte:head>
	<title>Currents</title>
	<meta name="description" content="A calm visual curation app on the AT Protocol." />
	<meta property="og:type" content="website" />
	<meta property="og:url" content={page.url.href} />
	<meta property="og:title" content="Currents" />
	<meta property="og:description" content="A calm visual curation app on the AT Protocol." />
	<meta name="twitter:card" content="summary" />
	<meta name="twitter:title" content="Currents" />
	<meta name="twitter:description" content="A calm visual curation app on the AT Protocol." />
</svelte:head>

{#if native}
	<MobileWelcome />
{:else}
	<LandingPage />
{/if}
