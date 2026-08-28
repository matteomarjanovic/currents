import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { SaveView } from './types';

const state = vi.hoisted(() => ({
	native: true,
	plugin: {
		download: vi.fn().mockResolvedValue(undefined),
		copyImage: vi.fn().mockResolvedValue(undefined),
		copyText: vi.fn().mockResolvedValue(undefined),
		shareLink: vi.fn().mockResolvedValue(undefined)
	}
}));

vi.mock('@capacitor/core', () => ({
	registerPlugin: () => state.plugin
}));
vi.mock('$lib/platform', () => ({
	isNative: () => state.native
}));
vi.mock('$lib/types', () => ({
	getImageContent: (save: SaveView) =>
		save.content.$type === 'is.currents.content.defs#imageView' ? save.content : null
}));
vi.mock('svelte-sonner', () => ({
	toast: { success: vi.fn(), error: vi.fn() }
}));

import { copyImage, copyLink, downloadImage, saveLink, shareLink } from './save-actions';

const save = {
	uri: 'at://did:plc:alice/is.currents.save/3abc',
	author: { did: 'did:plc:alice', handle: 'alice.test' },
	content: {
		$type: 'is.currents.content.defs#imageView',
		blobCid: 'bafkreiimage',
		imageUrl: 'https://cdn.currents.is/img/did:plc:alice/bafkreiimage'
	},
	createdAt: '2026-08-28T12:00:00Z'
} satisfies SaveView;

describe('native save actions', () => {
	beforeEach(() => {
		state.native = true;
		vi.clearAllMocks();
	});

	it('uses the public web URL for links from the native app', () => {
		expect(saveLink(save)).toBe('https://currents.is/profile/alice.test/save/3abc');
	});

	it('hands image downloads and copies to the native plugin', async () => {
		await downloadImage(save);
		await copyImage(save);

		const image = { url: save.content.imageUrl, fileName: '3abc' };
		expect(state.plugin.download).toHaveBeenCalledWith(image);
		expect(state.plugin.copyImage).toHaveBeenCalledWith(image);
	});

	it('shares and copies the public link through the native plugin', async () => {
		await shareLink(save);
		await copyLink(save);

		const url = 'https://currents.is/profile/alice.test/save/3abc';
		expect(state.plugin.shareLink).toHaveBeenCalledWith({ url });
		expect(state.plugin.copyText).toHaveBeenCalledWith({ text: url });
	});
});

describe('web save links', () => {
	it('keeps the current browser origin', () => {
		state.native = false;
		vi.stubGlobal('location', { origin: 'https://preview.example' });
		expect(saveLink(save)).toBe('https://preview.example/profile/alice.test/save/3abc');
		vi.unstubAllGlobals();
	});
});
