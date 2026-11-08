import { test, expect, type BrowserContext, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const IMAGE_URL = 'https://cdn.currents.is/img/did:plc:test/bafy1';

const save = {
	uri: 'at://did:plc:test/is.currents.save/s1',
	author: { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' },
	content: {
		$type: 'is.currents.content.defs#imageView',
		blobCid: 'bafy1',
		imageUrl: IMAGE_URL,
		width: 400,
		height: 500,
		alt: 'draggable reference'
	},
	createdAt: '2026-01-01T00:00:00Z',
	viewer: { saves: [] }
};

async function mockApi(page: Page, signedIn: boolean) {
	await page.route('https://cdn.currents.is/**', (route) =>
		route.fulfill({
			status: 200,
			contentType: 'image/svg+xml',
			body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500" />'
		})
	);
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (value: unknown) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(value)
			});

		if (url.includes('/api/me/role')) return json({ role: 'user' });
		if (url.includes('/api/me')) return json(signedIn ? save.author : null);
		if (url.includes('getFeed')) return json({ feed: [save], cursor: null });
		if (url.includes('getLibrarySaves')) return json({ saves: [save], cursor: null });
		if (url.includes('getActorCollections') || url.includes('getFavouriteCollections'))
			return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

for (const tall of [false, true]) {
	test(`desktop detail uses the loaded ${tall ? 'cropped' : 'portrait'} thumbnail until the larger image loads`, async ({
		page
	}, testInfo) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await mockApi(page, false);
		const imageSave = {
			...save,
			content: { ...save.content, height: tall ? 3200 : 500 }
		};
		await page.route(`${APPVIEW}/xrpc/is.currents.feed.getFeed*`, (route) =>
			route.fulfill({ json: { feed: [imageSave], cursor: null } })
		);
		let releaseDetail!: () => void;
		const detailReady = new Promise<void>((resolve) => {
			releaseDetail = resolve;
		});
		await page.route('https://cdn.currents.is/**', async (route) => {
			const detail = new URL(route.request().url()).searchParams.get('quality') === '85';
			if (detail) await detailReady;
			await route.fulfill({
				contentType: 'image/svg+xml',
				body: `<svg xmlns="http://www.w3.org/2000/svg" width="400" height="${detail ? imageSave.content.height : tall ? 800 : 500}"><rect width="100%" height="100%" fill="${detail ? 'red' : 'blue'}"/></svg>`
			});
		});
		try {
			await page.goto('/explore/general');
			const tile = page.locator('a.block:has(img)').first();
			const thumbnail = tile.locator('img').first();
			await expect
				.poll(() =>
					thumbnail.evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0)
				)
				.toBe(true);
			const thumbnailSrc = await thumbnail.evaluate((image: HTMLImageElement) => image.currentSrc);
			await tile.click();
			const detailsColumn = page
				.locator('[data-save-detail-overlay]')
				.getByRole('button', { name: 'Go back', exact: true })
				.first()
				.locator('..');
			await expect(detailsColumn).toHaveCSS('width', '400px');
			const preview = page.locator('[data-detail-preview]');
			await expect(preview).toBeVisible();
			await expect(preview).toHaveCSS('background-image', `url("${thumbnailSrc}")`);
			await expect(preview).toHaveCSS('background-size', tall ? '100% auto' : 'contain');
			const previewBox = (await preview.boundingBox())!;
			expect(previewBox.width).toBeGreaterThan(100);
			expect(previewBox.height).toBeGreaterThan(100);
			await page.screenshot({ path: testInfo.outputPath('thumbnail-placeholder.png') });
			const detailImage = page.locator('[data-save-detail-overlay] img[src*="quality=85"]').first();
			expect(await detailImage.evaluate((image: HTMLImageElement) => image.complete)).toBe(false);
			releaseDetail();
			await expect(page.locator('[data-detail-preview]')).toHaveCount(0);
			await expect
				.poll(() =>
					detailImage.evaluate(
						(image: HTMLImageElement) => image.complete && image.naturalWidth > 0
					)
				)
				.toBe(true);
		} finally {
			releaseDetail();
		}
	});
}

for (const path of ['/explore/general', '/organize']) {
	test(`a desktop ${path} image drags its original image URL`, async ({ browser, baseURL }) => {
		let context: BrowserContext | undefined;
		try {
			context = await browser.newContext({
				baseURL,
				viewport: { width: 1280, height: 800 },
				hasTouch: false,
				isMobile: false
			});
			const page = await context.newPage();
			await mockApi(page, path === '/organize');
			await page.goto(path);

			const image = page.getByAltText('draggable reference');
			await expect(image).toBeVisible();
			await expect(image).toHaveCSS('-webkit-user-drag', 'auto');

			await page.evaluate(() => {
				const dragWindow = window as typeof window & {
					dragResult?: { tag: string; uri: string; plain: string; types: string[] };
				};
				document.addEventListener(
					'dragstart',
					(event) => {
						dragWindow.dragResult = {
							tag: (event.target as HTMLElement).tagName,
							uri: event.dataTransfer?.getData('text/uri-list') ?? '',
							plain: event.dataTransfer?.getData('text/plain') ?? '',
							types: [...(event.dataTransfer?.types ?? [])]
						};
					},
					{ once: true }
				);
			});

			const box = await image.boundingBox();
			if (!box) throw new Error('missing image bounds');
			await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
			await page.mouse.down();
			await page.mouse.move(box.x + box.width / 2 + 80, box.y + box.height / 2 + 40, {
				steps: 10
			});
			await page.mouse.up();

			const dragResult = await page.evaluate(
				() =>
					(
						window as typeof window & {
							dragResult?: { tag: string; uri: string; plain: string; types: string[] };
						}
					).dragResult
			);
			expect(dragResult).toMatchObject({ tag: 'IMG', uri: IMAGE_URL, plain: IMAGE_URL });
			expect(dragResult?.types).toContain('Files');
		} finally {
			await context?.close();
		}
	});
}
