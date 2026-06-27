import { isNative } from './platform';
import { setAuthToken } from './auth-storage';
import { auth } from './stores/auth.svelte';
import { initShareTarget } from './share-target';

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

export async function initApp(): Promise<void> {
	if (initialized) return;
	initialized = true;
	if (!isNative()) return;

	const { App } = await import('@capacitor/app');
	// Status-bar icon color is handled reactively from the app theme in the root +layout.svelte
	// (via @capacitor-community/safe-area). The splash is hidden from there too, once content paints.

	App.addListener('appUrlOpen', async (event) => {
		try {
			const url = new URL(event.url);
			if (url.protocol !== 'currents:') return;
			// currents://oauth-callback?token=...&handle=...
			const path = (url.host || url.pathname.replace(/^\/+/, '')).split('/')[0];
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

	// Receive images/links shared to the app from the OS share sheet.
	initShareTarget();
}
