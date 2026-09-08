import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { isIos, isNative } from './platform';
import { mirrorAuthToken, setAuthToken } from './auth-storage';
import { auth } from './stores/auth.svelte';
import { loadCollections } from './stores/collections.svelte';
import { initShareTarget } from './share-target';
import { dismissTopOverlay } from './back-button';

let initialized = false;

export type DeepLinkEvent = { type: 'oauth-callback'; token: string; handle?: string };

const listeners = new Set<(ev: DeepLinkEvent) => void>();

export function onDeepLink(cb: (ev: DeepLinkEvent) => void): () => void {
	listeners.add(cb);
	return () => listeners.delete(cb);
}

function emit(ev: DeepLinkEvent) {
	for (const cb of listeners) cb(ev);
}

async function applyIosFontScale(): Promise<void> {
	try {
		const { AccessibilityPreferences } =
			await import('@capawesome/capacitor-accessibility-preferences');
		const { fontScale } = await AccessibilityPreferences.getPreferences();
		// Tailwind's rem scale assumes the browser's 16px default. WKWebView does not
		// apply Dynamic Type to that root size, so reproduce it from the native value.
		document.documentElement.style.fontSize = `${16 * fontScale}px`;
	} catch (err) {
		console.warn('Could not apply the iOS font-size preference', err);
	}
}

export async function initApp(): Promise<void> {
	if (initialized) return;
	initialized = true;
	if (!isNative()) return;

	// Keep the share extension's App Group token mirror fresh for installs that logged in
	// before the extension existed (see auth-storage.ts).
	void mirrorAuthToken();

	const { App } = await import('@capacitor/app');
	// Status-bar icon color is handled reactively from the app theme in the root +layout.svelte
	// (via @capacitor-community/safe-area). The splash is hidden from there too, once content paints.
	if (isIos()) {
		void applyIosFontScale();
		// Control Center makes the app inactive without necessarily backgrounding it.
		// Re-read on activation so its per-app text-size slider takes effect immediately.
		App.addListener('appStateChange', ({ isActive }) => {
			if (isActive) void applyIosFontScale();
		});
	}

	App.addListener('appUrlOpen', async (event) => {
		try {
			const url = new URL(event.url);
			if (url.protocol !== 'currents:') return;
			const path = (url.host || url.pathname.replace(/^\/+/, '')).split('/')[0];
			// Leave Android's disposable share activity for the normal app task before
			// entering organize mode, so switching apps doesn't destroy the details screen.
			if (path === 'organize') {
				const collectionUri = url.searchParams.get('c');
				await goto(
					collectionUri
						? `${resolve('/organize')}?c=${encodeURIComponent(collectionUri)}`
						: resolve('/organize')
				);
				return;
			}
			// currents://oauth-callback?token=...&handle=...
			if (path !== 'oauth-callback') return;
			const token = url.searchParams.get('token');
			const handle = url.searchParams.get('handle') ?? undefined;
			if (!token) return;
			await setAuthToken(token);
			auth.checked = false;
			try {
				const { Browser } = await import('@capacitor/browser');
				await Browser.close();
			} catch {
				// browser already closed
			}
			emit({ type: 'oauth-callback', token, handle });
		} catch (err) {
			console.warn('appUrlOpen handler error', err);
		}
	});

	// Android hardware back. Registering any listener switches off Capacitor's built-in
	// handling, so the navigate/exit fallback below has to be reproduced by hand.
	App.addListener('backButton', ({ canGoBack }) => {
		if (dismissTopOverlay()) return;
		if (canGoBack) history.back();
		else App.exitApp();
	});

	// Refresh the collections store when the app returns to the foreground: the iOS share
	// extension can create collections (and saves) while the webview sleeps, so the
	// launch-time load goes stale — the selector would miss a collection created from the
	// share sheet until a cold relaunch.
	App.addListener('resume', () => {
		if (auth.user) void loadCollections(auth.user.did);
	});

	// Receive images/links shared to the app from the OS share sheet.
	initShareTarget();
}
