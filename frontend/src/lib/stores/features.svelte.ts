import { PUBLIC_APPVIEW_URL } from '$env/static/public';
import { SvelteSet } from 'svelte/reactivity';

// Server-backed, per-user "seen feature" flags driving one-time "new feature"
// indicators. Keys are arbitrary strings; to announce a future feature, add a
// constant below, list it in ACTIVE_ANNOUNCEMENTS, and call markFeatureSeen()
// when the user engages with it — no backend change needed.

export const FEATURE_PINTEREST_IMPORT = 'pinterest-import';

// Features currently surfaced with a "new" indicator. Drop a key here once the
// feature is no longer newsworthy.
export const ACTIVE_ANNOUNCEMENTS = [FEATURE_PINTEREST_IMPORT];

export const features = $state({
	seen: new SvelteSet<string>(),
	loaded: false
});

export async function loadSeenFeatures() {
	try {
		const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/features/seen`, { credentials: 'include' });
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

// True when at least one announced feature hasn't been seen yet (drives the
// aggregate avatar dot). Only meaningful once features.loaded is true.
export function hasUnseenAnnouncement(): boolean {
	return ACTIVE_ANNOUNCEMENTS.some((k) => !features.seen.has(k));
}

export async function markFeatureSeen(key: string) {
	if (features.seen.has(key)) return;
	features.seen.add(key); // optimistic
	try {
		await fetch(`${PUBLIC_APPVIEW_URL}/api/features/seen/${encodeURIComponent(key)}`, {
			method: 'POST',
			credentials: 'include'
		});
	} catch {
		// best-effort; will resync on next load
	}
}
