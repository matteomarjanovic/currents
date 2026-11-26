<script lang="ts">
	import LandingPage from '$lib/components/landing-page.svelte';
	import MobileWelcome from '$lib/components/mobile-welcome.svelte';
	import { isNative } from '$lib/platform';

	const native = isNative();
</script>

{#if native}
	<MobileWelcome />
{:else}
	<LandingPage />
{/if}
