export type AuthState = 'authenticated' | 'unauthenticated';

export interface Collection {
  uri: string;
  name: string;
  saveCount: number;
}

// Module-level $state — shared between the content script message listener
// and the Svelte component. Vite bundles them into the same chunk so there
// is exactly one instance of this module per page.
export const clipper = $state({
  visible: false,
  imgUrl: '',
  originUrl: '',
  pageTitle: '',
  collections: [] as Collection[],
  authState: 'unauthenticated' as AuthState,
  userHandle: '',
});

export function showClipper(data: Omit<typeof clipper, 'visible'>) {
  Object.assign(clipper, data, { visible: true });
}

export function hideClipper() {
  clipper.visible = false;
}
