// A share received from the OS share sheet, waiting to be consumed: images → staged for
// upload; a link → scraped for images. `files` is a list because ACTION_SEND_MULTIPLE hands
// over a whole selection at once (see share-target.ts).
export type PendingShare = { type: 'image'; files: File[] } | { type: 'url'; url: string };

export const share = $state<{ pending: PendingShare | null }>({ pending: null });
