// Android's hardware back button. Capacitor's default is "go back in history, exit at the
// root", which ignores overlays — with the organize sidebar open, back left organize mode
// entirely instead of closing the sheet. Overlays register a dismisser while they're open;
// the most recently opened one wins, and with none registered we reproduce the default.
//
// The listener lives in app-init.ts and is native-only, so registering on web is inert.
const dismissers: (() => void)[] = [];

/** Close this overlay on the next back press. Call the returned function to deregister. */
export function onBackButton(dismiss: () => void): () => void {
	dismissers.push(dismiss);
	return () => {
		const i = dismissers.lastIndexOf(dismiss);
		if (i !== -1) dismissers.splice(i, 1);
	};
}

/** Dismiss the topmost overlay, if any. Returns false when the press should navigate instead. */
export function dismissTopOverlay(): boolean {
	const dismiss = dismissers.at(-1);
	if (!dismiss) return false;
	dismiss();
	return true;
}
