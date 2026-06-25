<script lang="ts">
	import '../layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { ModeWatcher } from 'mode-watcher';
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
		if (user) loadCollections(user.did);

		const isLoginPage = page.url.pathname.startsWith('/login');
		const isRegisterPage = page.url.pathname.startsWith('/register');
		const isLegalPage = page.url.pathname === '/terms' || page.url.pathname === '/privacy';
		const isRootPage = page.url.pathname === '/';
		const isExplorePage = page.url.pathname === '/explore';
		// Single-image (save-detail) pages are public — viewing an image must never
		// require login. Per-label moderation still applies (a logged-out viewer sees
		// the hidden-content state for labeled images), but the route itself is open.
		const isSavePage = page.url.pathname.includes('/save/');
		if (
			!user &&
			!isLoginPage &&
			!isRegisterPage &&
			!isLegalPage &&
			!isRootPage &&
			!isExplorePage &&
			!isSavePage
		) {
			goto(native ? '/' : '/login');
		}
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
<svelte:head><link rel="icon" href={favicon} /></svelte:head>

{#if !checked}
	<!-- loading -->
{:else if page.url.pathname === '/'}
	{@render children()}
{:else}
	<main>
		{@render children()}
	</main>
{/if}

{#if page.state.save}
	<div bind:this={overlayEl} class="fixed inset-0 z-50 overflow-y-auto app-muted-wash">
		<SaveDetail save={page.state.save} />
	</div>
{/if}

<LoginDialog />

<Toaster />
