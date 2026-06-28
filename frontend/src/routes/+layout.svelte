<script lang="ts">
	import { onMount } from 'svelte';
	import { dev } from '$app/environment';
	import { mode } from 'mode-watcher';
	import { initApp } from '$lib/app-init';
	import { isNative } from '$lib/platform';
	import { auth } from '$lib/stores/auth.svelte';
	// Side-effect import: registers the beforeinstallprompt listener on every page so the
	// one-shot event is captured even before the top bar (which offers "Install app") mounts.
	import '$lib/stores/pwa-install.svelte';

	let { children } = $props();

	onMount(() => {
		initApp().catch((err) => console.warn('initApp failed', err));
		// Register the PWA service worker in production web builds only. The same build is reused
		// by Capacitor, where a worker is unwanted (guard: isNative); and under `vite dev` the
		// worker is served on a different path, so we skip it there (test it via build/preview).
		if (!dev && !isNative() && 'serviceWorker' in navigator) {
			// updateViaCache 'none' keeps sw.js AND its importScripts (share-target-sw.js) fresh
			// on update checks instead of being served from the HTTP cache.
			navigator.serviceWorker.register('/sw.js', { updateViaCache: 'none' }).catch(() => {});
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
