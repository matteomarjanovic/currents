<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { ModeWatcher } from 'mode-watcher';
	import { isNative } from '$lib/platform';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import TopBar from '$lib/components/top-bar.svelte';
	import { Toaster } from '$lib/components/ui/sonner';

	let { children } = $props();

	// App Store rules forbid selling digital goods in-app through an external processor, so this
	// page (external Polar checkout) is hidden from the native apps entirely, same as /organize
	// while it's preview-gated.
	onMount(async () => {
		if (isNative()) {
			goto(resolve('/'));
			return;
		}
		// This route lives outside (with-navbar)/(without-navbar), so nothing else populates
		// auth.user for it — fetch it directly (a no-op if another layout already has).
		if (!auth.checked) {
			try {
				const res = await apiFetch('/api/me');
				if (res.ok) auth.user = await res.json();
			} catch {
				// appview unreachable
			}
			auth.checked = true;
		}
	});
</script>

<ModeWatcher />

<TopBar user={auth.user} />

{@render children()}

<Toaster />
