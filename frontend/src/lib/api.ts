import { PUBLIC_APPVIEW_URL } from '$env/static/public';
import { isNative } from './platform';
import { getAuthToken } from './auth-storage';

function resolveUrl(path: string): string {
	if (/^https?:\/\//.test(path)) return path;
	const base = PUBLIC_APPVIEW_URL.replace(/\/$/, '');
	return `${base}${path.startsWith('/') ? '' : '/'}${path}`;
}

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
	const url = resolveUrl(path);
	const headers = new Headers(init.headers ?? {});
	if (isNative()) {
		const token = await getAuthToken();
		if (token && !headers.has('Authorization')) {
			headers.set('Authorization', `Bearer ${token}`);
		}
		return fetch(url, { ...init, headers });
	}
	return fetch(url, { credentials: 'include', ...init, headers });
}
