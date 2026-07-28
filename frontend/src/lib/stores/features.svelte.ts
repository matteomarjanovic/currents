import { apiFetch } from '$lib/api';
import { SvelteSet } from 'svelte/reactivity';

// Server-backed, per-user "seen feature" flags driving one-time "new feature"
// indicators. Keys are arbitrary strings; to announce a future feature, add a
// constant below, list it in ACTIVE_ANNOUNCEMENTS, and call markFeatureSeen()
// when the user engages with it — no backend change needed.

export const FEATURE_PINTEREST_IMPORT = 'pinterest-import';
export const FEATURE_BLUESKY_IMPORT = 'bluesky-import';
export const FEATURE_BECOME_SUPPORTER = 'become-supporter';
export const FEATURE_ORGANIZE_MODE = 'organize-mode';
export const FEATURE_COLOR_SEARCH = 'color-search';

// The announcements currently live. Nothing reads this at runtime — each
// indicator checks its own key — but it's the one inventory of what's being
// announced, and appview/features.go mirrors it to decide what a brand-new
// account should be spared. Drop a key here once the feature is no longer
// newsworthy, and add it to onboardingSeenFeatures there once it's no longer
// worth greeting a newcomer with.
export const ACTIVE_ANNOUNCEMENTS = [
	FEATURE_PINTEREST_IMPORT,
	FEATURE_BLUESKY_IMPORT,
	FEATURE_BECOME_SUPPORTER,
	FEATURE_ORGANIZE_MODE,
	FEATURE_COLOR_SEARCH
];

export const features = $state({
	seen: new SvelteSet<string>(),
	loaded: false
});

export async function loadSeenFeatures() {
	try {
		const res = await apiFetch(`/api/features/seen`);
		if (!res.ok) return;
		const data = (await res.json()) as { seen?: string[] };
		// Union (flags are append-only) so an optimistic markFeatureSeen() done
		// before this load completes isn't clobbered by a stale response.
		for (const k of data.seen ?? []) features.seen.add(k);
		features.loaded = true;
	} catch {
		// best-effort; the indicator simply stays hidden until a later load
	}
}

export function isFeatureSeen(key: string): boolean {
	return features.seen.has(key);
}

export async function markFeatureSeen(key: string) {
	if (features.seen.has(key)) return;
	features.seen.add(key); // optimistic
	try {
		await apiFetch(`/api/features/seen/${encodeURIComponent(key)}`, {
			method: 'POST'
		});
	} catch {
		// best-effort; will resync on next load
	}
}
