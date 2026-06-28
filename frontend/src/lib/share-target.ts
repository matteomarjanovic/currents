import { goto } from '$app/navigation';
import { isNative } from './platform';
import { share, type PendingShare } from './stores/share.svelte';

// Receives shares from the OS (Android share sheet, via the `send-intent` plugin) and routes
// them to the upload page: an image is staged for upload, a link is fed to paste-from-URL.

function firstUrl(...vals: (string | undefined)[]): string | null {
	for (const v of vals) {
		const m = v?.match(/https?:\/\/\S+/i);
		if (m) return m[0];
	}
	return null;
}

function base64ToBytes(b64: string): Uint8Array<ArrayBuffer> {
	const bin = atob(b64);
	const bytes = new Uint8Array(new ArrayBuffer(bin.length));
	for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
	return bytes;
}

async function readSharedImage(uri: string, mime: string): Promise<File | null> {
	const { Filesystem } = await import('@capacitor/filesystem');
	const { data } = await Filesystem.readFile({ path: uri });
	if (typeof data !== 'string') return null; // base64 on native
	const ext = (mime.split('/')[1] || 'jpg').toLowerCase();
	return new File([base64ToBytes(data)], `shared-${Date.now()}.${ext}`, {
		type: mime || `image/${ext}`
	});
}

async function handleSharedIntent(): Promise<void> {
	const { SendIntent } = await import('send-intent');
	let result: { title?: string; description?: string; type?: string; url?: string };
	try {
		result = await SendIntent.checkSendIntentReceived();
	} catch {
		return; // nothing was shared
	}
	if (!result) return;

	const type = result.type ?? '';
	let next: PendingShare | null = null;
	try {
		if (type.startsWith('image/') && result.url) {
			const file = await readSharedImage(result.url, type);
			if (file) next = { type: 'image', file };
		} else {
			const url = firstUrl(result.url, result.description, result.title);
			if (url) next = { type: 'url', url };
		}
	} catch (err) {
		console.warn('share-target: failed to process share', err);
	}

	// NB: do NOT call SendIntent.finish() here — the plugin's SendIntentActivity is itself the
	// WebView/BridgeActivity hosting this share session, so finishing it would close the app
	// before the upload can happen. It closes on its own (onPause/onStop) when the user leaves.
	if (next) {
		share.pending = next;
		await goto('/upload');
	}
}

export function initShareTarget(): void {
	if (!isNative()) return;
	// A share that cold-launched the app.
	void handleSharedIntent();
	// A share received while the app is already running.
	window.addEventListener('sendIntentReceived', () => void handleSharedIntent());
}

// Web counterpart of handleSharedIntent: the installed PWA's service worker (static/
// share-target-sw.js) handles the OS share POST and redirects to /share-target with the
// shared data; this consumes it into share.pending and forwards to /upload, mirroring native.
// Keep SHARE_CACHE / SHARE_KEY in sync with static/share-target-sw.js.
const SHARE_CACHE = 'share-target';
const SHARE_KEY = '/__shared-image';

export async function consumeWebShare(): Promise<void> {
	const params = new URLSearchParams(window.location.search);
	try {
		if (params.get('shared') === 'image') {
			const cache = await caches.open(SHARE_CACHE);
			const res = await cache.match(SHARE_KEY);
			if (res) {
				await cache.delete(SHARE_KEY);
				const blob = await res.blob();
				const name = res.headers.get('x-filename') || `shared-${Date.now()}.jpg`;
				const file = new File([blob], name, { type: blob.type || 'image/jpeg' });
				share.pending = { type: 'image', file };
			}
		} else {
			const url = firstUrl(
				params.get('url') ?? undefined,
				params.get('text') ?? undefined,
				params.get('title') ?? undefined
			);
			if (url) share.pending = { type: 'url', url };
		}
	} catch (err) {
		console.warn('share-target: failed to process web share', err);
	}
	await goto('/upload', { replaceState: true });
}
