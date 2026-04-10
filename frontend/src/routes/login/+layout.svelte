<script lang="ts">
	import '../layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { onMount } from 'svelte';
	import { ModeWatcher } from 'mode-watcher';
	import TopBar from '$lib/components/top-bar.svelte';
	import { auth } from '$lib/stores/auth.svelte';

	let { children } = $props();

	let user: { did: string; handle: string; displayName?: string; avatar?: string } | null =
		$state(null);
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
		auth.user = user;
		checked = true;
		auth.checked = true;

		const isLoginPage = page.url.pathname.startsWith('/login');
		const isRootPage = page.url.pathname === '/';
		const isExplorePage = page.url.pathname === '/explore';
		if (!user && !isLoginPage && !isRootPage && !isExplorePage) {
			goto('/login');
		}
	});
</script>

<ModeWatcher />
<svelte:head><link rel="icon" href={favicon} /></svelte:head>

{#if !checked}
	<!-- loading -->
{:else}
	{#if page.url.pathname === '/'}
		{@render children()}
	{:else}
		{@render children()}
	{/if}
{/if}
