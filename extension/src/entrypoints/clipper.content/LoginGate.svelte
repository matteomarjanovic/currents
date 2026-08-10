<script lang="ts">
	import { clipper } from '../../lib/clipper-store.svelte';

	const LOGIN_PAGE_URL =
		import.meta.env.VITE_LOGIN_PAGE_URL ?? 'https://currents.is/login/extension';

	let waiting = $state(false);
	let pollIntervalId: ReturnType<typeof setInterval> | null = null;

	// Login happens in another tab, so the only way back is to poll.
	$effect(() => () => {
		if (pollIntervalId !== null) clearInterval(pollIntervalId);
	});

	function handleLoginClick() {
		waiting = true;
		pollIntervalId = setInterval(async () => {
			const res = await browser.runtime.sendMessage({ type: 'CHECK_AUTH' });
			if (!res.authenticated) return;
			if (pollIntervalId !== null) clearInterval(pollIntervalId);
			pollIntervalId = null;
			clipper.authState = 'authenticated';
			clipper.userHandle = res.handle;
			clipper.collections = res.collections;
			clipper.collectionsLoading = false;
		}, 3000);
	}
</script>

{#if waiting}
	<div class="flex flex-col items-center gap-3 py-2">
		<div class="size-6 animate-spin rounded-full border-3 border-muted border-t-foreground"></div>
		<p>Waiting for authentication…</p>
	</div>
{:else}
	<p class="pr-6">
		<a
			class="text-primary underline underline-offset-4"
			href={LOGIN_PAGE_URL}
			target="_blank"
			rel="noreferrer"
			onclick={handleLoginClick}
		>
			{clipper.reauthNeeded ? 'Reconnect to Currents' : 'Log in to Currents'}
		</a>
		{clipper.reauthNeeded ? 'to keep saving.' : 'to save images.'}
	</p>
{/if}
