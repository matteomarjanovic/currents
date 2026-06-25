import { apiFetch } from '$lib/api';

export type FollowerNotification = {
	did: string;
	handle: string;
	displayName?: string;
	avatar?: string;
	followedAt: string;
	youFollow: boolean;
	followUri?: string;
	isNew: boolean;
};

interface SocialState {
	loading: boolean;
	items: FollowerNotification[];
	unseenCount: number;
	cursor: string | undefined;
	hasMore: boolean;
	error: string | null;
}

/**
 * The Activity feed: who followed the current user. `unseenCount` drives the
 * unread dot (top-bar reads it); the dialog renders the full list. Marking the
 * tab seen zeroes `unseenCount` but leaves each item's `isNew` flag so the
 * highlight survives the current session.
 *
 * Not persisted — refetched via `refreshSocial()` on page load and on open.
 */
export const social = $state<SocialState>({
	loading: false,
	items: [],
	unseenCount: 0,
	cursor: undefined,
	hasMore: true,
	error: null
});

type SocialPage = { items: FollowerNotification[]; unseenCount: number; cursor?: string };

async function fetchPage(cursor?: string): Promise<SocialPage | null> {
	const params = new URLSearchParams({ limit: '30' });
	if (cursor) params.set('cursor', cursor);
	const res = await apiFetch(`/api/me/social?${params}`);
	if (res.status === 401) return null;
	if (!res.ok) throw new Error(`Failed to load (${res.status})`);
	return res.json();
}

export async function refreshSocial() {
	social.loading = true;
	social.error = null;
	try {
		const data = await fetchPage();
		if (!data) {
			social.items = [];
			social.unseenCount = 0;
			social.cursor = undefined;
			social.hasMore = false;
			return;
		}
		social.items = data.items ?? [];
		social.unseenCount = data.unseenCount ?? 0;
		social.cursor = data.cursor;
		social.hasMore = !!data.cursor;
	} catch (e) {
		social.error = String(e);
	} finally {
		social.loading = false;
	}
}

export async function loadMoreSocial() {
	if (social.loading || !social.hasMore || !social.cursor) return;
	social.loading = true;
	try {
		const data = await fetchPage(social.cursor);
		if (!data) return;
		const seen = new Set(social.items.map((i) => i.did));
		social.items = [...social.items, ...(data.items ?? []).filter((i) => !seen.has(i.did))];
		social.cursor = data.cursor;
		social.hasMore = !!data.cursor;
	} catch {
		social.hasMore = false;
	} finally {
		social.loading = false;
	}
}

export async function markSocialSeen() {
	social.unseenCount = 0;
	try {
		await apiFetch(`/api/me/social/seen`, {
			method: 'POST'
		});
	} catch {
		// best-effort; the dot reappears on next load if this failed
	}
}

export function clearSocial() {
	social.items = [];
	social.unseenCount = 0;
	social.cursor = undefined;
	social.hasMore = true;
	social.error = null;
}
