export type AuthState = 'authenticated' | 'unauthenticated';

export interface Collection {
  uri: string;
  name: string;
  saveCount: number;
}

export interface SiteHints {
  attributionCredit?: string;
}

interface ClipperState {
  visible: boolean;
  imgUrl: string;
  originUrl: string;
  pageTitle: string;
  collections: Collection[];
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
  authState: 'unauthenticated',
  userHandle: '',
  siteHints: {},
});

export function showClipper(data: Omit<typeof clipper, 'visible'>) {
  Object.assign(clipper, data, { visible: true });
}

export function hideClipper() {
  clipper.visible = false;
}
