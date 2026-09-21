import { registerPlugin } from '@capacitor/core';
import { completeOAuthCallback } from './app-init';
import { isAndroid } from './platform';

interface NativeAuthPlugin {
	isSupported(): Promise<{ value: boolean }>;
	open(options: { url: string; redirectScheme: string }): Promise<{ url: string }>;
}

const NativeAuth = registerPlugin<NativeAuthPlugin>('NativeAuth');

export function nativeOAuthReturnTo(): string {
	return isAndroid() ? 'is.currents.app://oauth-callback' : 'currents://oauth-callback';
}

export async function openNativeOAuth(url: string): Promise<void> {
	if (isAndroid()) {
		let supported = false;
		try {
			supported = (await NativeAuth.isSupported()).value;
		} catch {
			// Older builds do not contain the native plugin; keep their existing flow.
		}
		if (supported) {
			const result = await NativeAuth.open({ url, redirectScheme: 'is.currents.app' });
			if (!(await completeOAuthCallback(result.url))) {
				throw new Error('Authentication returned an invalid callback URL');
			}
			return;
		}
	}

	const { Browser } = await import('@capacitor/browser');
	await Browser.open({ url, presentationStyle: 'popover' });
}
