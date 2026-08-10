export type AuthState = 'authenticated' | 'unauthenticated';

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
	imgUrl: string;
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
	imgUrl: '',
	originUrl: '',
	pageTitle: '',
	collections: [],
	collectionsLoading: false,
	authState: 'unauthenticated',
	reauthNeeded: false,
	userHandle: '',
	siteHints: {}
});

export function showClipper(
	data: Omit<ClipperState, 'visible' | 'collectionsLoading' | 'reauthNeeded'> & {
		collectionsLoading?: boolean;
	}
) {
	Object.assign(clipper, { collectionsLoading: false, reauthNeeded: false }, data, {
		visible: true
	});
}

export function hideClipper() {
	clipper.visible = false;
}
