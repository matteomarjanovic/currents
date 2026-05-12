export type Browser = 'firefox' | 'safari' | 'chrome';

export function detectBrowser(): Browser {
	const ua = navigator.userAgent;
	if (ua.includes('Firefox/')) return 'firefox';
	if (ua.includes('Safari/') && !ua.includes('Chrome/') && !ua.includes('Chromium/')) return 'safari';
	return 'chrome';
}
