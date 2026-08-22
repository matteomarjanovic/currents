<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth.svelte';
	import { openSettings, type SettingsSection } from '$lib/stores/settings.svelte';

	// Deep-link: /settings/<section> opens the settings dialog (mounted once in the
	// root layout) over the explore feed. The (with-navbar) layout already bounces
	// logged-out users to /login and gates children on the auth check, so by the
	// time this mounts auth.user is resolved — we only handle the logged-in case.
	const SECTIONS: SettingsSection[] = ['account', 'feed', 'subscription', 'moderation'];

	onMount(() => {
		if (!auth.user) return;
		const raw = page.params.section;
		const section = SECTIONS.includes(raw as SettingsSection)
			? (raw as SettingsSection)
			: 'account';
		openSettings(section);
		goto(resolve('/(with-navbar)/explore'), { replaceState: true });
	});
</script>
