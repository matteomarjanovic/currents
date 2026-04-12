<script lang="ts">
	import '../layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { onMount } from 'svelte';
	import { ModeWatcher } from 'mode-watcher';
	import TopBar from '$lib/components/top-bar.svelte';
	import LoginDialog from '$lib/components/login-dialog.svelte';
	import SaveDetail from '$lib/components/save-detail.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { loadCollections } from '$lib/stores/collections.svelte';

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
		if (user) loadCollections(user.did);

		const isLoginPage = page.url.pathname.startsWith('/login');
		const isRootPage = page.url.pathname === '/';
		const isExplorePage = page.url.pathname === '/explore';
		if (!user && !isLoginPage && !isRootPage && !isExplorePage) {
			goto('/login');
		}
	});

	$effect(() => {
		if (page.state.save) {
			const prev = document.body.style.overflow;
			document.body.style.overflow = 'hidden';
			return () => {
				document.body.style.overflow = prev;
			};
		}
	});
</script>

<ModeWatcher />
<svelte:head><link rel="icon" href={favicon} /></svelte:head>

{#if !checked}
	<!-- loading -->
{:else}
	<TopBar {user} landing={page.url.pathname === '/'} />
	{#if page.url.pathname === '/'}
		{@render children()}
	{:else}
		<main class="p-4">
			{@render children()}
		</main>
	{/if}
{/if}

{#if page.state.save}
	<div class="fixed inset-0 z-50 overflow-y-auto bg-background">
		<SaveDetail save={page.state.save} />
	</div>
{/if}

<LoginDialog />
