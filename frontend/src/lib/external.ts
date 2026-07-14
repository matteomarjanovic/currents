import { isNative, isStandalonePwa } from './platform';

// Canonical public web origin. The blog is prerendered for SEO and lives outside the SPA
// (see src/routes/blog/+layout.ts); from inside the native app the webview origin isn't
// currents.is, so links to it must be absolute.
const WEB_ORIGIN = 'https://currents.is';

// True inside the native app or an installed (standalone) PWA — the contexts where we want
// prerendered marketing pages to open in the system browser instead of navigating in-app.
// A regular browser tab returns false, so normal in-page navigation is left untouched.
export function shouldOpenExternally(): boolean {
	return isNative() || isStandalonePwa();
}

// Open a same-site path in the system browser, leaving the native app / PWA shell.
export async function openExternal(path: string): Promise<void> {
	if (isNative()) {
		const { Browser } = await import('@capacitor/browser');
		await Browser.open({ url: WEB_ORIGIN + path });
		return;
	}
	// Installed PWA: a _blank window hands off to the system browser.
	window.open(window.location.origin + path, '_blank', 'noopener');
}
