export function isPinterestHost(hostname: string): boolean {
	return /(^|\.)pinterest\.[a-z.]+$/.test(hostname);
}

export function pinterestPinUrl(raw: string): string | undefined {
	try {
		const url = new URL(raw);
		const pin = url.pathname.match(/^\/pin\/(\d+)\/?$/)?.[1];
		if (url.protocol === 'https:' && isPinterestHost(url.hostname) && pin)
			return `${url.origin}/pin/${pin}/`;
	} catch {
		/* Not a pin URL. */
	}
}

export function externalSourceURL(raw?: string | null): string | undefined {
	if (!raw) return;
	try {
		const url = new URL(raw);
		if (
			!['http:', 'https:'].includes(url.protocol) ||
			isPinterestHost(url.hostname) ||
			url.hostname === 'pin.it' ||
			url.hostname.endsWith('.pin.it')
		)
			return;
		return url.href;
	} catch {
		/* Missing or malformed source URLs remain optional. */
	}
}

export function samePinterestPin(a: string, b: string): boolean {
	const first = pinterestPinUrl(a);
	const second = pinterestPinUrl(b);
	return !!first && !!second && new URL(first).pathname === new URL(second).pathname;
}

export function pinterestHtmlSource(html: string, pinUrl: string): string | undefined {
	const meta = new Map<string, string>();
	let clickthrough: string | undefined;
	for (const tag of html.match(/<(?:meta|a)\b[^>]*>/gi) ?? []) {
		const attrs = new Map(
			[...tag.matchAll(/([\w:-]+)\s*=\s*["']([^"']*)["']/g)].map((m) => [
				m[1],
				m[2].replace(/&amp;/g, '&')
			])
		);
		if (attrs.has('property') && attrs.has('content'))
			meta.set(attrs.get('property')!, attrs.get('content')!);
		if (!clickthrough && attrs.get('data-test-id') === 'clickthrough-link')
			clickthrough = externalSourceURL(attrs.get('href'));
	}
	const canonical = meta.get('og:url');
	if (canonical && !samePinterestPin(canonical, pinUrl)) return;
	return externalSourceURL(meta.get('og:see_also')) ?? clickthrough;
}

export async function resolvePinterestSource(pinUrl: string): Promise<string | undefined> {
	const pin = pinterestPinUrl(pinUrl);
	if (!pin) return;
	try {
		const response = await fetch(pin, { credentials: 'omit', signal: AbortSignal.timeout(5000) });
		if (response.ok) return pinterestHtmlSource(await response.text(), pin);
	} catch {
		/* Source lookup failure never prevents saving. */
	}
}
