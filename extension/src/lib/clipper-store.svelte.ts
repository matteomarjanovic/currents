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
  userHandle: '',
  siteHints: {},
});

export function showClipper(
  data: Omit<ClipperState, 'visible' | 'collectionsLoading'> & { collectionsLoading?: boolean }
) {
  Object.assign(clipper, { collectionsLoading: false }, data, { visible: true });
}

export function hideClipper() {
  clipper.visible = false;
}
