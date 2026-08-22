<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		feedPreferences,
		feedPreferencesLoaded,
		loadFeedPreferences
	} from '$lib/stores/feed-preferences.svelte';

	let redirecting = false;

	// Personalization lives in the route now. Logged-in visitors get their saved
	// default feed; everyone else gets the general one.
	$effect(() => {
		if (!auth.checked || redirecting) return;
		redirecting = true;
		if (!auth.user) {
			goto(resolve('/(with-navbar)/explore/[level]', { level: 'general' }), {
				replaceState: true
			});
			return;
		}
		void (async () => {
			if (!feedPreferencesLoaded.value) await loadFeedPreferences();
			await goto(
				resolve('/(with-navbar)/explore/[level]', { level: feedPreferences.defaultFeed }),
				{ replaceState: true }
			);
		})();
	});
</script>

<svelte:head>
	<title>Explore · Currents</title>
</svelte:head>
