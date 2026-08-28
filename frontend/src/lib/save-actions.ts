import { toast } from 'svelte-sonner';
import { registerPlugin } from '@capacitor/core';
import { getImageContent, type SaveView } from '$lib/types';
import { isNative } from '$lib/platform';

interface NativeSaveActionsPlugin {
	download(options: { url: string; fileName: string }): Promise<void>;
	copyImage(options: { url: string; fileName: string }): Promise<void>;
	copyText(options: { text: string }): Promise<void>;
	shareLink(options: { url: string }): Promise<void>;
}

const NativeSaveActions = registerPlugin<NativeSaveActionsPlugin>('NativeSaveActions');
const WEB_ORIGIN = 'https://currents.is';

function imageFileName(save: SaveView): string {
	return save.uri.split('/').pop() || 'currents-image';
}

// navigator.clipboard is only available in a secure context (https / localhost); over
// http on a LAN IP it's undefined, so fall back to a hidden-textarea execCommand copy.
export async function copyText(text: string): Promise<boolean> {
	if (isNative()) {
		try {
			await NativeSaveActions.copyText({ text });
			return true;
		} catch {
			return false;
		}
	}
	if (navigator.clipboard?.writeText) {
		try {
			await navigator.clipboard.writeText(text);
			return true;
		} catch {
			/* fall through to the legacy path */
		}
	}
	try {
		const ta = document.createElement('textarea');
		ta.value = text;
		ta.style.position = 'fixed';
		ta.style.top = '0';
		ta.style.opacity = '0';
		document.body.appendChild(ta);
		ta.focus();
		ta.select();
		const ok = document.execCommand('copy');
		ta.remove();
		return ok;
	} catch {
		return false;
	}
}

// The explore-mode detail URL for a save.
export function saveLink(save: SaveView): string {
	const rkey = save.uri.split('/').pop() ?? '';
	return `${isNative() ? WEB_ORIGIN : location.origin}/profile/${save.author.handle}/save/${rkey}`;
}

export async function copyLink(save: SaveView) {
	if (await copyText(saveLink(save))) toast.success('Link copied');
	else toast.error('Could not copy link');
}

async function encodePng(blob: Blob): Promise<Blob> {
	const bmp = await createImageBitmap(blob);
	const canvas = document.createElement('canvas');
	canvas.width = bmp.width;
	canvas.height = bmp.height;
	canvas.getContext('2d')!.drawImage(bmp, 0, 0);
	return new Promise((resolve, reject) =>
		canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('encode failed'))), 'image/png')
	);
}

export async function copyImage(save: SaveView) {
	const image = getImageContent(save);
	if (!image) return;
	try {
		if (isNative()) {
			await NativeSaveActions.copyImage({
				url: image.imageUrl,
				fileName: imageFileName(save)
			});
			toast.success('Image copied');
			return;
		}
		// cache: 'reload' bypasses the browser's immutable-cached copy of the image, which
		// was filled by an <img> load (no Origin) and so lacks the CORS header.
		const blob = await (await fetch(image.imageUrl, { cache: 'reload' })).blob();
		// The async clipboard only reliably accepts PNG for images; re-encode otherwise.
		const png = blob.type === 'image/png' ? blob : await encodePng(blob);
		await navigator.clipboard.write([new ClipboardItem({ 'image/png': png })]);
		toast.success('Image copied');
	} catch {
		toast.error('Could not copy image');
	}
}

export async function downloadImage(save: SaveView) {
	const image = getImageContent(save);
	if (!image) return;
	try {
		if (isNative()) {
			await NativeSaveActions.download({
				url: image.imageUrl,
				fileName: imageFileName(save)
			});
			toast.success('Image saved');
			return;
		}
		const blob = await (await fetch(image.imageUrl, { cache: 'reload' })).blob();
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		const ext = (blob.type.split('/')[1] || 'jpg').replace('jpeg', 'jpg');
		a.download = `${save.uri.split('/').pop()}.${ext}`;
		a.click();
		URL.revokeObjectURL(url);
	} catch {
		toast.error('Could not download image');
	}
}

export async function shareLink(save: SaveView) {
	const url = saveLink(save);
	try {
		if (isNative()) {
			await NativeSaveActions.shareLink({ url });
			return;
		}
		if (navigator.share) {
			await navigator.share({ url });
			return;
		}
		await copyLink(save);
	} catch (error) {
		if (error instanceof DOMException && error.name === 'AbortError') return;
		toast.error('Could not share link');
	}
}
