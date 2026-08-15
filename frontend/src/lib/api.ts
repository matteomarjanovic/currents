import { PUBLIC_APPVIEW_URL, PUBLIC_WEB_APPVIEW_URL } from '$env/static/public';
import { isNative } from './platform';
import { getAuthToken } from './auth-storage';

export function appviewUrl(path: string): string {
	if (/^https?:\/\//.test(path)) return path;
	const base = (isNative() ? PUBLIC_APPVIEW_URL : PUBLIC_WEB_APPVIEW_URL).replace(/\/$/, '');
	return `${base}${path.startsWith('/') ? '' : '/'}${path}`;
}

export function logoutUrl(): string {
	if (isNative() || PUBLIC_WEB_APPVIEW_URL) return appviewUrl('/oauth/logout');
	return `${PUBLIC_APPVIEW_URL.replace(/\/$/, '')}/oauth/logout/legacy`;
}

export async function apiFetch(
	path: string,
	init: RequestInit = {},
	fetcher: typeof fetch = fetch
): Promise<Response> {
	const url = appviewUrl(path);
	const headers = new Headers(init.headers ?? {});
	if (isNative()) {
		const token = await getAuthToken();
		if (token && !headers.has('Authorization')) {
			headers.set('Authorization', `Bearer ${token}`);
		}
		return fetcher(url, { ...init, headers });
	}
	return fetcher(url, { credentials: 'include', ...init, headers });
}
