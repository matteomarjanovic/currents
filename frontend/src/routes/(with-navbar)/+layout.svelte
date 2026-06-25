<script lang="ts">
	import '../layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { ModeWatcher } from 'mode-watcher';
	import TopBar from '$lib/components/top-bar.svelte';
	import LoginDialog from '$lib/components/login-dialog.svelte';
	import SaveDetail from '$lib/components/save-detail.svelte';
	import { Toaster } from '$lib/components/ui/sonner';
	import { auth } from '$lib/stores/auth.svelte';
	import { loadCollections } from '$lib/stores/collections.svelte';
	import { apiFetch } from '$lib/api';
	import { isNative } from '$lib/platform';
	import { lockBodyScroll } from '$lib/scroll-lock';

	let { children } = $props();

	const native = isNative();

	let user: { did: string; handle: string; displayName?: string; avatar?: string } | null =
		$state(null);
	let checked = $state(false);

	onMount(async () => {
		try {
			const res = await apiFetch('/api/me');
			if (res.ok) {
				user = await res.json();
			}
		} catch {
			// appview unreachable
		}
		auth.user = user;
		checked = true;
		auth.checked = true;
		if (user) {
			loadCollections(user.did);
			if (page.url.pathname === '/') goto('/explore');
		}

		const isPublic =
			page.url.pathname === '/' ||
			page.url.pathname.startsWith('/explore') ||
			page.url.pathname.startsWith('/login') ||
			page.url.pathname.startsWith('/register') ||
			page.url.pathname.startsWith('/profile/') ||
			page.url.pathname.startsWith('/collection/');
		if (!user && !isPublic) {
			// On native the login entry point is the welcome screen at '/', not the web /login route.
			goto(native ? '/' : '/login');
		}
	});

	$effect(() => {
		if (auth.checked && auth.user && page.url.pathname === '/') goto('/explore');
	});

	$effect(() => {
		if (page.state.save) return lockBodyScroll();
	});

	let overlayEl: HTMLDivElement | undefined = $state();
	const overlayScroll = new Map<string, number>();
	let trackedUri: string | undefined;
	let restoring = false;

	$effect(() => {
		if (!overlayEl) return;
		const el = overlayEl;
		const onScroll = () => {
			if (!restoring && trackedUri) overlayScroll.set(trackedUri, el.scrollTop);
		};
		el.addEventListener('scroll', onScroll, { passive: true });
		return () => el.removeEventListener('scroll', onScroll);
	});

	$effect(() => {
		const uri = page.state.save?.uri as string | undefined;
		trackedUri = uri;
		if (!overlayEl || !uri) return;
		const el = overlayEl;
		const target = overlayScroll.get(uri) ?? 0;
		restoring = true;
		el.scrollTop = target;
		const start = performance.now();
		const tick = () => {
			if (!el.isConnected || trackedUri !== uri) {
				restoring = false;
				return;
			}
			if (el.scrollTop < target - 1) el.scrollTop = target;
			if (el.scrollTop < target - 1 && performance.now() - start < 3000) {
				requestAnimationFrame(tick);
			} else {
				restoring = false;
			}
		};
		requestAnimationFrame(tick);
	});
</script>

<ModeWatcher />
<svelte:head>
	<link rel="icon" href={favicon} />
	{#if page.url.pathname === '/'}
		<title>Currents</title>
		<meta name="description" content="A calm visual curation app on the AT Protocol." />
		<meta property="og:type" content="website" />
		<meta property="og:url" content={page.url.href} />
		<meta property="og:title" content="Currents" />
		<meta property="og:description" content="A calm visual curation app on the AT Protocol." />
		<meta name="twitter:card" content="summary" />
		<meta name="twitter:title" content="Currents" />
		<meta name="twitter:description" content="A calm visual curation app on the AT Protocol." />
	{/if}
</svelte:head>

{#if !checked}
	<!-- loading -->
{:else}
	{#if !(native && page.url.pathname === '/')}
		<TopBar {user} landing={page.url.pathname === '/'} />
	{/if}
	{#if page.url.pathname === '/' && !auth.user}
		{@render children()}
	{:else if page.url.pathname !== '/'}
		<main class="p-2 md:p-4">
			{@render children()}
		</main>
	{/if}
{/if}

{#if page.state.save}
	<div bind:this={overlayEl} class="fixed inset-0 z-50 overflow-y-auto app-muted-wash">
		<SaveDetail save={page.state.save} />
	</div>
{/if}

<LoginDialog />

<Toaster />
