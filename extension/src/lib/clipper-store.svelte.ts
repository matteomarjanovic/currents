export type AuthState = 'authenticated' | 'unauthenticated';

export interface Collection {
  uri: string;
  name: string;
  saveCount: number;
}

// Module-level $state — shared between the content script message listener
// and the Svelte component. Vite bundles them into the same chunk so there
// is exactly one instance of this module per page.
export interface SiteHints {
  attributionCredit?: string;
}

export type ClipperMode = 'single' | 'board';

interface ClipperState {
  visible: boolean;
  mode: ClipperMode;
  imgUrl: string;
  originUrl: string;
  pageTitle: string;
  collections: Collection[];
  authState: AuthState;
  userHandle: string;
  siteHints: SiteHints;
  pinCount: number;
  defaultCollectionDescription: string;
}

export const clipper: ClipperState = $state({
  visible: false,
  mode: 'single',
  imgUrl: '',
  originUrl: '',
  pageTitle: '',
  collections: [],
  authState: 'unauthenticated',
  userHandle: '',
  siteHints: {},
  pinCount: 0,
  defaultCollectionDescription: '',
});

export function showClipper(data: Omit<typeof clipper, 'visible'>) {
  Object.assign(clipper, data, { visible: true });
}

export function hideClipper() {
  clipper.visible = false;
}
