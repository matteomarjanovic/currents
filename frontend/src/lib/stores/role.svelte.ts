import { PUBLIC_PREVIEW_FEATURES } from '$env/static/public';
import { apiFetch } from '$lib/api';

// The viewer's moderation role (null = regular user), mirrored from
// GET /api/me/role. Loaded on demand by surfaces that gate on it.
export const role = $state({
	value: null as string | null,
	loaded: false
});

export async function loadRole() {
	try {
		const res = await apiFetch('/api/me/role');
		if (!res.ok) return;
		role.value = ((await res.json()) as { role: string | null }).role;
		role.loaded = true;
	} catch {
		// appview unreachable; treated as no role
	}
}

// Staged-rollout gate: while PUBLIC_PREVIEW_FEATURES=1, unreleased features
// (organize mode, the supporter/subscription UI) are visible only to
// moderators. Clear the variable and rebuild to make them public — the
// backend needs no change either way.
export const previewGated = PUBLIC_PREVIEW_FEATURES === '1';

export function canSeePreviewFeatures(): boolean {
	return !previewGated || role.value != null;
}
