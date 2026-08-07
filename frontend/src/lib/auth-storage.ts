import { PUBLIC_APPVIEW_URL } from '$env/static/public';
import { registerPlugin } from '@capacitor/core';
import { isIos, isNative } from './platform';

const TOKEN_KEY = 'currents_auth_token';

// Mirrors the token (+ appview base URL) into the App Group container so the iOS share
// extension can call the appview itself — it can't read the app's keychain entry. Implemented
// by SharedAuthPlugin in ios/App/App/AppDelegate.swift; iOS-only.
const SharedAuth = registerPlugin<{
	set(options: { token: string; apiUrl: string }): Promise<void>;
	clear(): Promise<void>;
}>('SharedAuth');

export async function getAuthToken(): Promise<string | null> {
	if (!isNative()) return null;
	const { SecurePreferences } = await import('@capawesome-team/capacitor-secure-preferences');
	const { value } = await SecurePreferences.get({ key: TOKEN_KEY });
	return value ?? null;
}

export async function setAuthToken(token: string): Promise<void> {
	if (!isNative()) return;
	const { SecurePreferences } = await import('@capawesome-team/capacitor-secure-preferences');
	await SecurePreferences.set({ key: TOKEN_KEY, value: token });
	if (isIos()) await SharedAuth.set({ token, apiUrl: PUBLIC_APPVIEW_URL }).catch(() => {});
}

export async function clearAuthToken(): Promise<void> {
	if (!isNative()) return;
	const { SecurePreferences } = await import('@capawesome-team/capacitor-secure-preferences');
	await SecurePreferences.remove({ key: TOKEN_KEY });
	if (isIos()) await SharedAuth.clear().catch(() => {});
}

// Upgrade path for installs that logged in before the share extension existed: re-mirror the
// keychain token on launch (cheap, idempotent). Called from initApp.
export async function mirrorAuthToken(): Promise<void> {
	if (!isIos()) return;
	const token = await getAuthToken();
	if (token) await SharedAuth.set({ token, apiUrl: PUBLIC_APPVIEW_URL }).catch(() => {});
}
