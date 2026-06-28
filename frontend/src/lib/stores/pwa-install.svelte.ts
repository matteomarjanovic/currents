// Captures the browser's `beforeinstallprompt` so the avatar menu can offer a custom
// "Install app" action that triggers the native PWA install (Chromium: Android/desktop).
// iOS has no programmatic install, so the UI falls back to instructions there.
//
// The listeners are registered at module load. The root layout side-effect-imports this
// module so registration happens on every page — `beforeinstallprompt` is a one-shot event
// and is easy to miss if we wait for a specific component to mount.

interface BeforeInstallPromptEvent extends Event {
	prompt: () => Promise<void>;
	userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

let deferred: BeforeInstallPromptEvent | null = null;

export const pwaInstall = $state({
	// A beforeinstallprompt has been captured and not yet used (Chromium only).
	canPrompt: false,
	// The app was installed during this session (appinstalled fired) — hide the affordance.
	installed: false
});

if (typeof window !== 'undefined') {
	window.addEventListener('beforeinstallprompt', (e) => {
		// Suppress Chrome's default mini-infobar; we present our own button instead.
		e.preventDefault();
		deferred = e as BeforeInstallPromptEvent;
		pwaInstall.canPrompt = true;
	});
	window.addEventListener('appinstalled', () => {
		deferred = null;
		pwaInstall.canPrompt = false;
		pwaInstall.installed = true;
	});
}

/** Trigger the native install prompt. Returns false if no prompt is available. */
export async function promptInstall(): Promise<boolean> {
	if (!deferred) return false;
	const e = deferred;
	deferred = null; // the event can only be used once
	pwaInstall.canPrompt = false;
	await e.prompt();
	return true;
}
