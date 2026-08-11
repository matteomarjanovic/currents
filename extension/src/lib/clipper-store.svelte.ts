import { browser } from 'wxt/browser';
import type { ImageCandidate } from '../entrypoints/clipper.content/collect-images';

export type AuthState = 'authenticated' | 'unauthenticated';

// 'single' is one image picked by hand (right-click, or the Pinterest button);
// 'multi' is every image found on the page, shown as a grid to pick from.
export type ClipperMode = 'single' | 'multi';

export interface Collection {
	uri: string;
	name: string;
	saveCount: number;
	parentUri?: string;
	previews?: { url: string; labels?: string[] }[];
	createdAt?: string;
	lastSavedAt?: string;
}

export interface SiteHints {
	attributionCredit?: string;
	originUrl?: string;
}

interface ClipperState {
	visible: boolean;
	mode: ClipperMode;
	// Bumped on every open. The pickers are keyed on it, so reopening remounts
	// them with fresh form state instead of carrying the last image's over.
	session: number;
	// Set while a batch save is in flight: the run lives in this component, so
	// dismissing the dialog would kill it halfway through.
	locked: boolean;
	imgUrl: string;
	candidates: ImageCandidate[];
	originUrl: string;
	pageTitle: string;
	collections: Collection[];
	collectionsLoading: boolean;
	authState: AuthState;
	// True when authState flipped to unauthenticated because the session lacks the
	// uploadBlob rpc: scope (not a real logout) — the UI shows "reconnect" copy.
	reauthNeeded: boolean;
	userHandle: string;
	siteHints: SiteHints;
}

export const clipper: ClipperState = $state({
	visible: false,
	mode: 'single',
	session: 0,
	locked: false,
	imgUrl: '',
	candidates: [],
	originUrl: '',
	pageTitle: '',
	collections: [],
	collectionsLoading: false,
	authState: 'unauthenticated',
	reauthNeeded: false,
	userHandle: '',
	siteHints: {}
});

// Opens the dialog immediately, optimistically assuming a signed-in user with
// collections still loading — every caller then kicks off loadClipperAuth() so
// the dialog never waits on a network round-trip to appear.
export function showClipper(data: Partial<ClipperState>) {
	Object.assign(
		clipper,
		{
			mode: 'single',
			locked: false,
			imgUrl: '',
			candidates: [],
			originUrl: '',
			pageTitle: '',
			collections: [],
			collectionsLoading: true,
			authState: 'authenticated',
			reauthNeeded: false,
			userHandle: '',
			siteHints: {}
		},
		data,
		{ visible: true, session: clipper.session + 1 }
	);
}

export function hideClipper() {
	clipper.visible = false;
}

export async function loadClipperAuth() {
	const session = clipper.session;
	const res = await browser.runtime.sendMessage({ type: 'CHECK_AUTH' });
	// Dismissed, or superseded by a later open, while the request was in flight.
	if (!clipper.visible || clipper.session !== session) return;
	clipper.authState = res.authenticated ? 'authenticated' : 'unauthenticated';
	clipper.collections = res.authenticated ? res.collections : [];
	clipper.userHandle = res.authenticated ? res.handle : '';
	clipper.collectionsLoading = false;
}

// The collection that received the most recent save; on ties (a save in a
// section also bumps its root) prefer the section, then the newest collection.
export function defaultCollectionUri(cols: Collection[]): string {
	const best = [...cols].sort((a, b) => {
		const ra = a.lastSavedAt ? Date.parse(a.lastSavedAt) : 0;
		const rb = b.lastSavedAt ? Date.parse(b.lastSavedAt) : 0;
		if (rb !== ra) return rb - ra;
		if (!!a.parentUri !== !!b.parentUri) return a.parentUri ? -1 : 1;
		const ca = a.createdAt ? Date.parse(a.createdAt) : 0;
		const cb = b.createdAt ? Date.parse(b.createdAt) : 0;
		return cb - ca;
	});
	return best[0]?.uri ?? '';
}
