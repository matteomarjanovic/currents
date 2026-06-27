<script lang="ts">
	import { onMount } from 'svelte';
	import { mode } from 'mode-watcher';
	import { initApp } from '$lib/app-init';
	import { isNative } from '$lib/platform';
	import { auth } from '$lib/stores/auth.svelte';

	let { children } = $props();

	onMount(() => {
		initApp().catch((err) => console.warn('initApp failed', err));
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
</script>

{@render children()}
