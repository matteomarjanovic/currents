import { apiFetch } from '$lib/api';

export type AttestationItem = {
	id: number;
	source: string;
	subjectUri: string;
	blobCid?: string;
	category?: string;
	labelVal?: string;
	score?: number;
	priority: number;
	status: string;
	createdAt: string;
	previewUrl?: string;
	disputed?: boolean;
	disputedAt?: string;
};

interface NotificationsState {
	loading: boolean;
	items: AttestationItem[];
	error: string | null;
}

/**
 * In-memory cache of the current user's pending self-attestations. Top-bar
 * reads `items.length` for the bell-icon badge; the dialog reads the full
 * list and writes back via `refresh()` after any confirm/dispute action.
 *
 * Not persisted — refetched on every page load via `refresh()` from the
 * (with-navbar) layout right after the auth check completes.
 */
export const notifications = $state<NotificationsState>({
	loading: false,
	items: [],
	error: null
});

export async function refreshNotifications() {
	notifications.loading = true;
	notifications.error = null;
	try {
		const res = await apiFetch(`/api/me/attestations`);
		if (res.status === 401) {
			notifications.items = [];
			return;
		}
		if (!res.ok) {
			notifications.error = `Failed to load (${res.status})`;
			return;
		}
		const data = (await res.json()) as { items: AttestationItem[] };
		notifications.items = data.items ?? [];
	} catch (e) {
		notifications.error = String(e);
	} finally {
		notifications.loading = false;
	}
}

export function clearNotifications() {
	notifications.items = [];
	notifications.error = null;
	notifications.loading = false;
}
