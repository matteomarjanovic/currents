<script lang="ts">
	import '../layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { onMount, untrack } from 'svelte';
	import { ModeWatcher } from 'mode-watcher';
	import TopBar from '$lib/components/top-bar.svelte';
	import LoginDialog from '$lib/components/login-dialog.svelte';
	import SaveDetailOverlay from '$lib/components/save-detail-overlay.svelte';
	import { Toaster } from '$lib/components/ui/sonner';
	import { auth } from '$lib/stores/auth.svelte';
	import { loadCollections } from '$lib/stores/collections.svelte';
	import { apiFetch } from '$lib/api';
	import { isNative } from '$lib/platform';
	import { lockBodyScroll } from '$lib/scroll-lock';
	import { activateSaveSequence } from '$lib/save-sequence.svelte';
	import type { LayoutData } from './$types';

	let { data, children }: { data: LayoutData; children: import('svelte').Snippet } = $props();

	const native = isNative();

	// The home route. Match on route.id, not `pathname === '/'`: the native webview serves the
	// SPA fallback as `/index.html`, so the pathname isn't literally `/` there — a string compare
	// would leave a logged-in user stuck on the welcome screen (no redirect) with the nav bar shown.
	const isHome = $derived(page.route.id === '/(with-navbar)');
	const isPublic = $derived(
		isHome ||
			page.url.pathname.startsWith('/explore') ||
			page.url.pathname.startsWith('/login') ||
			page.url.pathname.startsWith('/register') ||
			page.url.pathname.startsWith('/profile/') ||
			page.url.pathname.startsWith('/collection/') ||
			// Text search reads fine unauthenticated (searchSaves uses optional auth); the color route
			// still 403s, which raises the login prompt in-page.
			page.url.pathname.startsWith('/search/')
	);

	let user: { did: string; handle: string; displayName?: string; avatar?: string } | null = $state(
		untrack(() => data.viewer)
	);
	let checked = $state(untrack(() => !!data.viewer));

	onMount(async () => {
		if (!user) {
			try {
				const res = await apiFetch('/api/me');
				if (res.ok) {
					user = await res.json();
				}
			} catch {
				// appview unreachable
			}
		}
		auth.user = user;
		checked = true;
		auth.checked = true;
		if (user) {
			loadCollections(user.did);
		}

		if (!user && !isPublic) {
			// On native the login entry point is the welcome screen at '/', not the web /login route.
			goto(native ? resolve('/') : resolve('/login'));
		}
	});

	$effect(() => {
		// The home route is only a signed-in entry point, not a distinct history stop. Replacing
		// it lets browser Back return to the page that preceded Currents instead of bouncing
		// through / and immediately entering Explore again.
		if (auth.checked && auth.user && isHome) {
			goto(resolve('/(with-navbar)/explore'), { replaceState: true });
		}
	});

	let saveStack = $derived(page.state.saveStack ?? (page.state.save ? [page.state.save] : []));
	let saveOverlayOpen = $derived(saveStack.length > 0);
	let activeSaveDepth = $derived(saveStack.length - 1);
	$effect(() => {
		if (saveOverlayOpen) return lockBodyScroll();
	});
	$effect(() => activateSaveSequence(activeSaveDepth));
</script>

<ModeWatcher />
<svelte:head>
	<link rel="icon" href={favicon} />
	{#if isHome}
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

{#if !checked && !isPublic}
	<!-- loading -->
{:else}
	{#if !(native && isHome)}
		<TopBar {user} landing={isHome} />
	{/if}
	{#if isHome && !auth.user}
		{@render children()}
	{:else if !isHome}
		<main class="p-2 md:p-4">
			{@render children()}
		</main>
	{/if}
{/if}

{#each saveStack as save, i (i)}
	<SaveDetailOverlay {save} active={i === activeSaveDepth} />
{/each}

<LoginDialog />

<Toaster />
