import { beforeEach, describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({
	android: true,
	supported: true,
	callbackUrl: 'is.currents.app://oauth-callback?token=opaque',
	complete: vi.fn().mockResolvedValue(true),
	nativeOpen: vi.fn(),
	browserOpen: vi.fn()
}));

vi.mock('@capacitor/core', () => ({
	registerPlugin: () => ({
		isSupported: async () => ({ value: state.supported }),
		open: state.nativeOpen
	})
}));

vi.mock('./app-init', () => ({ completeOAuthCallback: state.complete }));
vi.mock('./platform', () => ({ isAndroid: () => state.android }));
vi.mock('@capacitor/browser', () => ({ Browser: { open: state.browserOpen } }));

import { nativeOAuthReturnTo, openNativeOAuth } from './native-auth';

describe('openNativeOAuth', () => {
	beforeEach(() => {
		state.android = true;
		state.supported = true;
		state.nativeOpen.mockReset().mockResolvedValue({ url: state.callbackUrl });
		state.browserOpen.mockReset().mockResolvedValue(undefined);
		state.complete.mockReset().mockResolvedValue(true);
	});

	it('uses Android Auth Tab and completes its returned callback', async () => {
		await openNativeOAuth('https://api.currents.is/oauth/login');
		expect(state.nativeOpen).toHaveBeenCalledWith({
			url: 'https://api.currents.is/oauth/login',
			redirectScheme: 'is.currents.app'
		});
		expect(state.complete).toHaveBeenCalledWith(state.callbackUrl);
		expect(state.browserOpen).not.toHaveBeenCalled();
	});

	it('uses an app-specific Android return scheme', () => {
		expect(nativeOAuthReturnTo()).toBe('is.currents.app://oauth-callback');
		state.android = false;
		expect(nativeOAuthReturnTo()).toBe('currents://oauth-callback');
	});

	it('keeps the existing browser flow when Auth Tab is unavailable', async () => {
		state.supported = false;
		await openNativeOAuth('https://api.currents.is/oauth/login');
		expect(state.nativeOpen).not.toHaveBeenCalled();
		expect(state.browserOpen).toHaveBeenCalledOnce();
	});
});
