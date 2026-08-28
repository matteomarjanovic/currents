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

async function mockApi(page: Page) {
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

		if (url.includes('/api/me')) return json(null);
		if (url.includes('getFeed')) return json({ feed: [save], cursor: null });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

test('a desktop grid image drags its original image URL', async ({ browser }) => {
	let context: BrowserContext | undefined;
	try {
		context = await browser.newContext({
			baseURL: 'http://localhost:5173',
			viewport: { width: 1280, height: 800 },
			hasTouch: false,
			isMobile: false
		});
		const page = await context.newPage();
		await mockApi(page);
		await page.goto('/explore/general');

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
