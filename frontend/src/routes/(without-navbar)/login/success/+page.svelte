<script lang="ts">
	import { onMount } from 'svelte';
	import LogoMerged from '$lib/assets/logo.svelte';

	let seconds = $state(5);

	onMount(() => {
		const id = setInterval(() => {
			seconds -= 1;
			if (seconds <= 0) {
				clearInterval(id);
				// Best-effort: the extension closes this tab from its background
				// script (a page can't close a user-opened tab itself).
				window.close();
			}
		}, 1000);
		return () => clearInterval(id);
	});
</script>

<svelte:head>
	<title>Login successful · Currents</title>
</svelte:head>

<div class="flex min-h-svh flex-col items-center justify-center gap-6 p-6 text-center md:p-10">
	<div class="flex h-8 items-center">
		<LogoMerged />
	</div>
	<div class="flex flex-col gap-2">
		<h1 class="text-lg font-medium">Login successful</h1>
		<p class="text-sm text-muted-foreground">
			{#if seconds > 0}
				This tab will close in {seconds}…
			{:else}
				You can close this tab.
			{/if}
		</p>
	</div>
</div>
