import { browser } from 'wxt/browser';

const KEY = 'saveButtonSettings';
const defaults = { allSites: false, domains: ['pinterest.com'] };

export const saveButtonSettings = $state({
	...defaults,
	domains: [...defaults.domains],
	ready: false
});

export function isPinterestHost(hostname: string): boolean {
	return /(^|\.)pinterest\.[a-z.]+$/.test(hostname);
}

export function isCurrentsHost(hostname: string): boolean {
	const host = hostname.toLowerCase().replace(/^www\./, '');
	return host === 'currents.is' || host.endsWith('.currents.is');
}

export function normalizeDomain(input: string): string | null {
	const raw = input.trim().toLowerCase();
	if (!raw || /\s/.test(raw)) return null;
	try {
		const url = new URL(raw.includes('://') ? raw : `https://${raw}`);
		if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return null;
		const host = url.hostname.replace(/^www\./, '');
		if (host !== 'localhost' && !host.includes('.')) return null;
		if (!/^[a-z0-9.-]+$/.test(host) || host.startsWith('.') || host.endsWith('.')) return null;
		if (host.split('.').some((part) => !part || part.startsWith('-') || part.endsWith('-')))
			return null;
		return host;
	} catch {
		return null;
	}
}

export function showButtonOnSite(hostname: string): boolean {
	if (isCurrentsHost(hostname)) return false;
	if (saveButtonSettings.allSites) return true;
	const host = hostname.toLowerCase().replace(/^www\./, '');
	return saveButtonSettings.domains.some(
		(domain) =>
			host === domain ||
			host.endsWith(`.${domain}`) ||
			(domain === 'pinterest.com' && isPinterestHost(host))
	);
}

export async function loadSaveButtonSettings() {
	browser.storage.onChanged.addListener((changes, area) => {
		if (area === 'local' && changes[KEY])
			Object.assign(saveButtonSettings, changes[KEY].newValue ?? defaults);
	});
	const saved = await browser.storage.local.get(KEY);
	Object.assign(saveButtonSettings, saved[KEY] ?? defaults, { ready: true });
}

export function updateSaveButtonSettings(next: { allSites: boolean; domains: string[] }) {
	const value = { allSites: next.allSites, domains: [...next.domains] };
	Object.assign(saveButtonSettings, value);
	return browser.storage.local.set({ [KEY]: value });
}
