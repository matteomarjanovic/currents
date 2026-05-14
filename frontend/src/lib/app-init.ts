import { isNative } from './platform';
import { setAuthToken } from './auth-storage';
import { auth } from './stores/auth.svelte';

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
	try {
		const { StatusBar, Style } = await import('@capacitor/status-bar');
		StatusBar.setStyle({ style: Style.Default }).catch(() => {});
	} catch {
		// status-bar plugin not available
	}
	try {
		const { SplashScreen } = await import('@capacitor/splash-screen');
		SplashScreen.hide().catch(() => {});
	} catch {
		// splash-screen plugin not available
	}

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
}
