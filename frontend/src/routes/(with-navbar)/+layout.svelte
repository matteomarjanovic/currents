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
		if (user) {
			loadCollections(user.did);
			if (page.url.pathname === '/') goto('/explore');
		}

		const isLoginPage = page.url.pathname.startsWith('/login');
		const isRegisterPage = page.url.pathname.startsWith('/register');
		const isRootPage = page.url.pathname === '/';
		const isExplorePage = page.url.pathname === '/explore';
		if (!user && !isLoginPage && !isRegisterPage && !isRootPage && !isExplorePage) {
			goto('/login');
		}
	});

	$effect(() => {
		if (auth.checked && auth.user && page.url.pathname === '/') goto('/explore');
	});

	$effect(() => {
		if (page.state.save) {
			const y = window.scrollY;
			const body = document.body;
			const prev = {
				position: body.style.position,
				top: body.style.top,
				left: body.style.left,
				right: body.style.right,
				width: body.style.width,
				overflow: body.style.overflow
			};
			body.style.position = 'fixed';
			body.style.top = `-${y}px`;
			body.style.left = '0';
			body.style.right = '0';
			body.style.width = '100%';
			body.style.overflow = 'hidden';
			return () => {
				body.style.position = prev.position;
				body.style.top = prev.top;
				body.style.left = prev.left;
				body.style.right = prev.right;
				body.style.width = prev.width;
				body.style.overflow = prev.overflow;
				window.scrollTo(0, y);
			};
		}
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
	<TopBar {user} landing={page.url.pathname === '/'} />
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
