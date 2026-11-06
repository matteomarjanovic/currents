<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { dev } from '$app/env';
	import { afterNavigate, onNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { mode } from 'mode-watcher';
	import { initApp } from '$lib/app-init';
	import { isIos, isNative } from '$lib/platform';
	import { auth } from '$lib/stores/auth.svelte';
	import { findFeedLevel } from '$lib/feed-levels';
	import { exploreFeedScope, lastExploreFeed, peekExploreFeed } from '$lib/explore-feed';
	import { feedPreferences } from '$lib/stores/feed-preferences.svelte';
	import {
		waitForExploreFeedReturn,
		finishExploreFeedReturn
	} from '$lib/stores/explore-feed-restoration.svelte';
	import { loadModerationPrefs, modPrefsLoaded } from '$lib/stores/moderation-prefs.svelte';
	import { loadPreferences, preferences, preferencesLoaded } from '$lib/stores/preferences.svelte';
	import { setExploreSaveSuggestionMode } from '$lib/stores/save-suggestions.svelte';
	import SettingsDialog from '$lib/components/settings-dialog.svelte';
	import SupporterDialog from '$lib/components/supporter-dialog.svelte';
	import SupporterThanksDialog from '$lib/components/supporter-thanks-dialog.svelte';
	import SaveRemovalDialog from '$lib/components/save-removal-dialog.svelte';
	import ModeTabs from '$lib/components/mode-tabs.svelte';
	import { supporterGate } from '$lib/stores/supporter.svelte';
	import { initAnalytics, trackPendingAuthSuccess } from '$lib/analytics';
	// Side-effect import: registers the beforeinstallprompt listener on every page so the
	// one-shot event is captured even before the top bar (which offers "Install app") mounts.
	import '$lib/stores/pwa-install.svelte';

	let { children } = $props();

	let feedReturnTransition: ViewTransition | undefined;
	let finishFeedNavigation = () => {};
	onNavigate((navigation) => {
		if (navigation.shallow) return;
		const to = navigation.to;
		const entry = to?.route.id === '/(with-navbar)/explore';
		const destination =
			to?.route.id === '/(with-navbar)/explore/[level]' && to.url.pathname.startsWith('/explore/');
		if (feedReturnTransition) {
			const transition = feedReturnTransition;
			const finish = finishFeedNavigation;
			if (destination)
				navigation.complete.then(finish, () => {
					finish();
					transition.skipTransition();
				});
			else if (!entry) {
				finish();
				transition.skipTransition();
			}
			return;
		}
		if ((!entry && !destination) || !document.startViewTransition) return;
		const scope = exploreFeedScope(auth.user?.did ?? '', feedPreferences.excludedCollections);
		const level = entry ? lastExploreFeed(scope) : findFeedLevel(to?.params?.level)?.slug;
		// A same-level route update reuses the live grid; it has no restore to await.
		if (
			destination &&
			navigation.from?.route.id === to?.route.id &&
			navigation.from?.params?.level === level
		)
			return;
		if (!level || !peekExploreFeed(scope, level)?.items.length) return;
		const gridReady = waitForExploreFeedReturn();
		const navigationReady = new Promise<void>((resolve) => {
			finishFeedNavigation = resolve;
		});
		const finish = finishFeedNavigation;
		return new Promise<void>((continueNavigation) => {
			document.documentElement.dataset.exploreFeedReturn = '';
			const transition = document.startViewTransition(async () => {
				continueNavigation();
				await navigationReady;
			});
			feedReturnTransition = transition;
			if (destination)
				navigation.complete.then(finish, () => {
					finish();
					transition.skipTransition();
				});
			// Await grid frames only after the update callback: view transitions pause
			// requestAnimationFrame during that callback, which masonry needs to settle.
			Promise.all([transition.ready, gridReady]).then(
				() => transition.skipTransition(),
				() => transition.skipTransition()
			);
			const cleanup = () => {
				finishExploreFeedReturn();
				feedReturnTransition = undefined;
				delete document.documentElement.dataset.exploreFeedReturn;
			};
			transition.finished.then(cleanup, cleanup);
		});
	});

	afterNavigate((navigation) => {
		if (navigation.shallow) return;
		if (page.status >= 400 && feedReturnTransition) {
			finishFeedNavigation();
			feedReturnTransition.skipTransition();
		}
		if (!isIos()) return;
		// SvelteKit switches history entries to manual scroll restoration after mounting.
		// WKWebView then rejects a back-swipe snapshot when that entry was captured at a
		// different scroll position, leaving the gesture blank. Let WebKit restore scroll
		// in the iOS shell too; SvelteKit's later restoration remains a harmless fallback.
		requestAnimationFrame(() => {
			history.scrollRestoration = 'auto';
		});
	});

	onMount(() => {
		initAnalytics();
		initApp().catch((err) => console.warn('initApp failed', err));
		// Register the PWA service worker in production web builds only. The same build is reused
		// by Capacitor, where a worker is unwanted (guard: isNative); and under `vite dev` the
		// worker is served on a different path, so we skip it there (test it via build/preview).
		if (!dev && !isNative() && 'serviceWorker' in navigator) {
			// Always check the bundled worker for updates without an HTTP cache hit.
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

	// Viewer preferences that drive how content renders — moderation blur/hide and GIF autoplay —
	// are loaded here, the only layout every route shares. They used to be fetched from the top
	// bar, which the standalone save pages and organize mode never render: landing on one of those
	// directly (a refresh, a shared link) left both stores at their defaults, so a viewer who had
	// set adult content to "show" saw it blurred, and one who had set it to "hide" saw it blurred
	// instead of gone. Fires again on login, since the effect tracks `auth.user`.
	$effect(() => {
		if (!auth.user) return;
		untrack(() => {
			trackPendingAuthSuccess();
			if (!modPrefsLoaded.value) void loadModerationPrefs();
			if (!preferencesLoaded.value) void loadPreferences();
		});
	});

	$effect(() => {
		setExploreSaveSuggestionMode(preferences.saveSuggestionMode);
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

<!-- One persistent desktop instance: route layouts only provide its destination anchor. -->
<ModeTabs viewTransitionName="mode-switcher" />

<!-- Mounted once at the root so every mode (explore, organize, save pages) can
     open it via the settingsDialog store. -->
<SettingsDialog />

<!-- Paywall, mounted once so any supporter-gated action (color search,
     library search, find similar) can raise it via requireSupporter. -->
<SupporterDialog bind:open={supporterGate.open} />

<!-- Post-checkout thank-you, opened from $lib/polar.ts wherever the checkout
     was started (paywall dialog, settings, support page). -->
<SupporterThanksDialog />

<!-- Last-copy guard shared by collection menus and organize mode. -->
<SaveRemovalDialog />
