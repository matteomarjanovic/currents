import { isNative } from './platform';

const TOKEN_KEY = 'currents_auth_token';

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
}

export async function clearAuthToken(): Promise<void> {
	if (!isNative()) return;
	const { SecurePreferences } = await import('@capawesome-team/capacitor-secure-preferences');
	await SecurePreferences.remove({ key: TOKEN_KEY });
}
