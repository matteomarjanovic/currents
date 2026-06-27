// A share received from the OS share sheet (Android ACTION_SEND), waiting for the upload page
// to consume it: a shared image → staged for upload; a shared link → fed to paste-from-URL.
export type PendingShare = { type: 'image'; file: File } | { type: 'url'; url: string };

export const share = $state<{ pending: PendingShare | null }>({ pending: null });
