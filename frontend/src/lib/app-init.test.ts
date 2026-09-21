import { beforeEach, describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({
	user: { did: 'did:plc:test', handle: 'test.bsky.social' },
	auth: { user: null as { did: string; handle: string } | null, checked: true },
	setToken: vi.fn(),
	apiFetch: vi.fn(),
	loadCollections: vi.fn(),
	closeBrowser: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));
vi.mock('./platform', () => ({ isNative: () => true, isIos: () => false }));
vi.mock('./auth-storage', () => ({ setAuthToken: state.setToken, mirrorAuthToken: vi.fn() }));
vi.mock('./api', () => ({ apiFetch: state.apiFetch }));
vi.mock('./stores/auth.svelte', () => ({ auth: state.auth }));
vi.mock('./stores/collections.svelte', () => ({ loadCollections: state.loadCollections }));
vi.mock('./share-target', () => ({ initShareTarget: vi.fn() }));
vi.mock('@capacitor/browser', () => ({ Browser: { close: state.closeBrowser } }));

import { completeOAuthCallback, onDeepLink } from './app-init';

beforeEach(() => {
	state.auth.user = null;
	state.auth.checked = true;
	state.setToken.mockReset().mockResolvedValue(undefined);
	state.apiFetch.mockReset().mockImplementation(async () => Response.json(state.user));
	state.loadCollections.mockReset().mockResolvedValue(undefined);
	state.closeBrowser.mockReset().mockResolvedValue(undefined);
});

describe('completeOAuthCallback', () => {
	it.each(['currents', 'is.currents.app'])('signs in the mounted app for %s', async (scheme) => {
		state.apiFetch.mockImplementation(async () => {
			expect(state.setToken).toHaveBeenCalledWith('new-token');
			return Response.json(state.user);
		});
		const onLogin = vi.fn(() => {
			expect(state.auth.user).toEqual(state.user);
			expect(state.auth.checked).toBe(true);
		});
		const unsubscribe = onDeepLink(onLogin);
		try {
			expect(
				await completeOAuthCallback(
					`${scheme}://oauth-callback?token=new-token&handle=test.bsky.social`
				)
			).toBe(true);
			expect(state.apiFetch).toHaveBeenCalledWith('/api/me');
			expect(state.loadCollections).toHaveBeenCalledWith(state.user.did);
			expect(state.closeBrowser).toHaveBeenCalledOnce();
			expect(onLogin).toHaveBeenCalledOnce();
		} finally {
			unsubscribe();
		}
	});

	it('refreshes an existing session even when the authentication browser is already closed', async () => {
		state.auth.user = { ...state.user, handle: 'old.bsky.social' };
		state.closeBrowser.mockRejectedValueOnce(new Error('No browser open'));
		expect(await completeOAuthCallback('is.currents.app://oauth-callback?token=new-token')).toBe(
			true
		);
		expect(state.auth.user).toEqual(state.user);
		expect(state.auth.checked).toBe(true);
	});

	it('does not announce a successful login when the new token is rejected', async () => {
		state.apiFetch.mockResolvedValueOnce(new Response(null, { status: 401 }));
		const onLogin = vi.fn();
		const unsubscribe = onDeepLink(onLogin);
		try {
			await expect(
				completeOAuthCallback('currents://oauth-callback?token=bad-token')
			).rejects.toThrow('Could not load the authenticated profile');
			expect(state.auth.user).toBeNull();
			expect(state.auth.checked).toBe(true);
			expect(onLogin).not.toHaveBeenCalled();
		} finally {
			unsubscribe();
		}
	});

	it.each([
		'https://currents.is/oauth-callback?token=web-token',
		'currents://organize?token=wrong-path',
		'is.currents.app://oauth-callback'
	])('ignores a non-login callback: %s', async (url) => {
		expect(await completeOAuthCallback(url)).toBe(false);
		expect(state.setToken).not.toHaveBeenCalled();
		expect(state.apiFetch).not.toHaveBeenCalled();
	});
});
