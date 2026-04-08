<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { onMount } from 'svelte';
	import { ModeWatcher } from 'mode-watcher';
	import TopBar from '$lib/components/top-bar.svelte';

	let { children } = $props();

	let user: { did: string; handle: string; displayName?: string; avatar?: string } | null = $state(null);
	let checked = $state(false);

	onMount(async () => {
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/me`, { credentials: 'include' });
			if (res.ok) {
				user = await res.json();
			}
		} catch {
			// appview unreachable
		}
		checked = true;

		if (!user && !page.url.pathname.startsWith('/login')) {
			goto('/login');
		}
	});
</script>

<ModeWatcher />
<svelte:head><link rel="icon" href={favicon} /></svelte:head>

{#if !checked}
	<!-- loading -->
{:else if user}
	<TopBar {user} />
	<main class="p-4">
		{@render children()}
	</main>
{:else}
	{@render children()}
{/if}
