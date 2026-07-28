<script lang="ts">
	import { onMount } from 'svelte';
	import { afterNavigate } from '$app/navigation';
	import { dev } from '$app/environment';
	import { mode } from 'mode-watcher';
	import { initApp } from '$lib/app-init';
	import { isNative } from '$lib/platform';
	import { auth } from '$lib/stores/auth.svelte';
	import { navHistory } from '$lib/stores/navigation.svelte';
	import SettingsDialog from '$lib/components/settings-dialog.svelte';
	import SupporterDialog from '$lib/components/supporter-dialog.svelte';
	import SupporterThanksDialog from '$lib/components/supporter-thanks-dialog.svelte';
	import { supporterGate } from '$lib/stores/supporter.svelte';
	// Side-effect import: registers the beforeinstallprompt listener on every page so the
	// one-shot event is captured even before the top bar (which offers "Install app") mounts.
	import '$lib/stores/pwa-install.svelte';

	let { children } = $props();

	// `from` is only null on the tab's very first navigation — every navigation after that was
	// pushed by SvelteKit itself, so the back button is guaranteed to land on Currents.
	afterNavigate(({ from }) => {
		if (from) navHistory.hasInternalHistory = true;
	});

	onMount(() => {
		initApp().catch((err) => console.warn('initApp failed', err));
		// Register the PWA service worker in production web builds only. The same build is reused
		// by Capacitor, where a worker is unwanted (guard: isNative); and under `vite dev` the
		// worker is served on a different path, so we skip it there (test it via build/preview).
		if (!dev && !isNative() && 'serviceWorker' in navigator) {
			// updateViaCache 'none' keeps sw.js AND its importScripts (share-target-sw.js) fresh
			// on update checks instead of being served from the HTTP cache.
			navigator.serviceWorker
				.register('/sw.js', { updateViaCache: 'none' })
				.then((reg) => {
					// One-time migration off the worker deployed before the vite.config.ts fix: it was
					// generated without skipWaiting, so its replacement parks in "waiting" — still serving
					// the old worker's cache-first precache — until every tab on the origin is closed.
					// Nudging it through is the only way those clients pick up the fix; the replacement
					// skips waiting on its own, so this is a no-op once everyone has moved over and can
					// be deleted then.
					const nudge = () => reg.waiting?.postMessage({ type: 'SKIP_WAITING' });
					nudge();
					reg.addEventListener('updatefound', () =>
						reg.installing?.addEventListener('statechange', nudge)
					);
				})
				.catch(() => {});
		}
	});

	// Hide the native splash only once the first content is ready (the route layouts gate rendering
	// on auth.checked), after a paint, then fade — otherwise the splash cuts to a blank frame.
	let splashHidden = false;
	$effect(() => {
		if (splashHidden || !auth.checked || !isNative()) return;
		splashHidden = true;
		requestAnimationFrame(() =>
			requestAnimationFrame(() =>
				import('@capacitor/splash-screen')
					.then(({ SplashScreen }) => SplashScreen.hide({ fadeOutDuration: 200 }))
					.catch(() => {})
			)
		);
	});

	// Keep the native system-bar icon colors in sync with the APP's theme (not the phone's), so
	// the status/navigation bar icons stay legible on the top bar in both light and dark mode.
	$effect(() => {
		const m = mode.current;
		if (!isNative() || m === undefined) return;
		import('@capacitor-community/safe-area')
			.then(({ SafeArea, SystemBarsStyle }) =>
				SafeArea.setSystemBarsStyle({
					style: m === 'dark' ? SystemBarsStyle.Dark : SystemBarsStyle.Light
				})
			)
			.catch(() => {});
	});

	// Web/installed-PWA counterpart: match the browser/status-bar tint to the app's --background
	// per theme. A single dynamic meta (not prefers-color-scheme media tags) is required because
	// the theme can be manually overridden (light/dark/system) independent of the OS. Hex values
	// mirror --background in src/routes/layout.css.
	$effect(() => {
		const m = mode.current;
		if (isNative() || m === undefined) return;
		document
			.querySelector('meta[name="theme-color"]')
			?.setAttribute('content', m === 'dark' ? '#090b0c' : '#ffffff');
	});
</script>

{@render children()}

<!-- Mounted once at the root so every mode (explore, organize, save pages) can
     open it via the settingsDialog store. -->
<SettingsDialog />

<!-- Paywall, mounted once so any supporter-gated action (color search,
     library search, find similar) can raise it via requireSupporter. -->
<SupporterDialog bind:open={supporterGate.open} />

<!-- Post-checkout thank-you, opened from $lib/polar.ts wherever the checkout
     was started (paywall dialog, settings, support page). -->
<SupporterThanksDialog />
