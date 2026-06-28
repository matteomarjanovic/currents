<script lang="ts">
	import { onMount } from 'svelte';
	import { consumeWebShare } from '$lib/share-target';

	// The service worker redirects the OS share POST here with the shared data; hand it off to
	// share.pending and forward to /upload. Top-level route (root layout only) so there's no
	// navbar flash during the brief redirect.
	onMount(() => {
		void consumeWebShare();
	});
</script>

<div class="flex min-h-svh items-center justify-center text-sm text-muted-foreground">Opening…</div>
