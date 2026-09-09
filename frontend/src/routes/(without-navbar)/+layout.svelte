<script lang="ts">
	import '../layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount, untrack } from 'svelte';
	import { ModeWatcher } from 'mode-watcher';
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

	let user: { did: string; handle: string; displayName?: string; avatar?: string } | null = $state(
		untrack(() => data.viewer)
	);
	let checked = $state(untrack(() => !!data.viewer));
	const isPublic = $derived(
		page.url.pathname.startsWith('/login') ||
			page.url.pathname.startsWith('/register') ||
			page.url.pathname === '/terms' ||
			page.url.pathname === '/privacy' ||
			page.url.pathname === '/refunds' ||
			page.url.pathname === '/' ||
			page.url.pathname === '/explore' ||
			// Single-image pages are public. Per-label moderation still applies, but viewing a shared
			// image must not require authentication.
			page.url.pathname.includes('/save/')
	);

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
		if (user) loadCollections(user.did);

		if (!user && !isPublic) {
			goto(native ? '/' : '/login');
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
<svelte:head><link rel="icon" href={favicon} /></svelte:head>

{#if !checked && !isPublic}
	<!-- loading -->
{:else if page.url.pathname === '/'}
	{@render children()}
{:else}
	<main>
		{@render children()}
	</main>
{/if}

{#each saveStack as save, i (i)}
	<SaveDetailOverlay {save} active={i === activeSaveDepth} />
{/each}

<LoginDialog />

<Toaster />
