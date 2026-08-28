import { apiFetch } from '$lib/api';
import type { OrganizeCollectionSort } from '$lib/organize-collections';

// Reactive viewer preferences for UI/rendering behavior. Server-backed so they
// follow the user across browsers and devices (web + mobile). Distinct from
// moderation-prefs, which gate content visibility.
interface Prefs {
	// Autoplay animated GIFs. When false, GIFs render frozen at their first frame
	// and play on hover (see save-image.svelte).
	gifAutoplay: boolean;
	organizeCollectionSort: OrganizeCollectionSort;
}

// Defaults mirror the appview DB column defaults (migrations 042 and 048).
const DEFAULTS: Prefs = {
	gifAutoplay: true,
	organizeCollectionSort: 'name'
};

export const preferences = $state<Prefs>({ ...DEFAULTS });
export const preferencesLoaded = $state({ value: false });

export async function loadPreferences() {
	try {
		const res = await apiFetch(`/api/preferences`);
		if (!res.ok) return;
		const data = (await res.json()) as Partial<Prefs>;
		if (typeof data.gifAutoplay === 'boolean') preferences.gifAutoplay = data.gifAutoplay;
		if (data.organizeCollectionSort === 'name' || data.organizeCollectionSort === 'recent') {
			preferences.organizeCollectionSort = data.organizeCollectionSort;
		}
		preferencesLoaded.value = true;
	} catch {
		// best-effort; the defaults remain in effect until a later load
	}
}

async function persist() {
	try {
		await apiFetch(`/api/preferences`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				gifAutoplay: preferences.gifAutoplay,
				organizeCollectionSort: preferences.organizeCollectionSort
			})
		});
	} catch {
		// best-effort; will resync from the server on next load
	}
}

export function setGifAutoplay(val: boolean) {
	preferences.gifAutoplay = val; // optimistic
	void persist();
}

export function setOrganizeCollectionSort(val: OrganizeCollectionSort) {
	preferences.organizeCollectionSort = val; // optimistic
	void persist();
}
