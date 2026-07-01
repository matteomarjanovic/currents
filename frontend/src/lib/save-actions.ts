import { toast } from 'svelte-sonner';
import { getImageContent, type SaveView } from '$lib/types';

// navigator.clipboard is only available in a secure context (https / localhost); over
// http on a LAN IP it's undefined, so fall back to a hidden-textarea execCommand copy.
export async function copyText(text: string): Promise<boolean> {
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
	return `${location.origin}/profile/${save.author.handle}/save/${rkey}`;
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
