const UTM_KEYS = ['utm_source', 'utm_medium', 'utm_campaign', 'utm_content', 'utm_term'] as const;

function normalizedPath(pathname: string): string {
	const parts = pathname.split('/').filter(Boolean);
	if (parts[0] === 'search' && parts.length >= 3) return `/search/${parts[1]}/:query`;
	if (parts[0] === 'profile' && parts.length >= 2) {
		if (parts[2] === 'collection' && parts[3]) return '/profile/:actor/collection/:collection';
		if (parts[2] === 'save' && parts[3]) return '/profile/:actor/save/:save';
		return '/profile/:actor';
	}
	if (parts[0] === 'collection' && parts[1]) return '/collection/:collection';
	if (parts[0] === 'save' && parts[1]) return '/save/:save';
	return pathname;
}

// Analytics needs campaign attribution, not user content. Keep only standard
// UTM fields and collapse routes whose path itself contains a query or AT URI.
export function sanitizeAnalyticsUrl(raw: string, base = 'https://currents.is'): string {
	try {
		const parsed = new URL(raw, base);
		const query = new URLSearchParams();
		for (const key of UTM_KEYS) {
			const value = parsed.searchParams.get(key);
			if (value) query.set(key, value);
		}
		const suffix = query.size ? `?${query}` : '';
		return normalizedPath(parsed.pathname) + suffix;
	} catch {
		return '/';
	}
}

export function sanitizeAnalyticsReferrer(raw: string, ownUrl: string): string {
	if (!raw) return '';
	try {
		const own = new URL(ownUrl);
		const parsed = new URL(raw, own);
		if (parsed.protocol === own.protocol && parsed.host === own.host) {
			return sanitizeAnalyticsUrl(parsed.href, own.href);
		}
		return parsed.origin;
	} catch {
		return '';
	}
}

export function shouldTrackAnalyticsPath(raw: string, base = 'https://currents.is'): boolean {
	try {
		const path = new URL(raw, base).pathname;
		return !path.startsWith('/admin') && !path.startsWith('/moderation');
	} catch {
		return false;
	}
}
