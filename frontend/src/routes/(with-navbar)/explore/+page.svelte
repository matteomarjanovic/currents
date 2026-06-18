<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';

	// Personalization lives in the route now. Logged-in visitors get their
	// personal feed; everyone else gets the general one.
	$effect(() => {
		if (!auth.checked) return;
		goto(auth.user ? '/explore/personal' : '/explore/general', { replaceState: true });
	});
</script>
