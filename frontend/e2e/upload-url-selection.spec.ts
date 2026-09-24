import { test, expect } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const collectionUri = 'at://did:plc:test/is.currents.feed.collection/ideas';
const imageUrls = [400, 800, 1200].map((width) => `${APPVIEW}/img/page/${width}.svg`);

test.use({ viewport: { width: 1280, height: 800 }, hasTouch: false, isMobile: false });

test('a pasted page URL uploads only selected images and shows their resolutions', async ({
	page
}) => {
	const uploads: string[] = [];
	await page.route(`${APPVIEW}/**`, (route) => {
		const request = route.request();
		const url = request.url();
		const json = (value: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(value) });
		if (url.includes('/img/page/')) {
			const width = Number(url.match(/\/(\d+)\.svg$/)?.[1]);
			return route.fulfill({
				status: 200,
				contentType: 'image/svg+xml',
				body: `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="500"/>`
			});
		}
		if (url.endsWith('/api/extract-images')) return json({ images: imageUrls });
		if (url.endsWith('/api/save') && request.method() === 'POST') {
			uploads.push(request.postData() ?? '');
			return json({ uri: `at://did:plc:test/is.currents.feed.save/new${uploads.length}` });
		}
		if (url.includes('/api/me/role')) return json({ role: 'user' });
		if (url.includes('/api/me')) return json({ did: 'did:plc:test', handle: 'test.bsky.social' });
		if (url.includes('getActorCollections'))
			return json({
				collections: [{ uri: collectionUri, name: 'Ideas', createdAt: '2026-01-01T00:00:00Z' }]
			});
		if (url.includes('getFavouriteCollections')) return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('/api/preferences')) return json({});
		return json({});
	});
	await page.goto('/upload');
	await page.getByPlaceholder('…or paste a page or image URL').fill('https://example.com/gallery');
	await page.getByRole('button', { name: 'Fetch', exact: true }).click();

	const start = page.getByRole('button', { name: 'Start upload' });
	await expect(page.getByText('0 of 3 images selected')).toBeVisible();
	await expect(page.getByRole('button', { name: 'Remove image' })).toHaveCount(0);
	await expect(start).toBeDisabled();
	const first = page.getByRole('button', { name: 'Image, 400 by 500 pixels' });
	const middle = page.getByRole('button', { name: 'Image, 800 by 500 pixels' });
	const last = page.getByRole('button', { name: 'Image, 1200 by 500 pixels' });
	await expect(first).toHaveAttribute('aria-pressed', 'false');
	await expect(page.getByText('400×500')).toBeVisible();
	await expect(page.getByText('800×500')).toBeVisible();
	await expect(page.getByText('1200×500')).toBeVisible();
	await first.click();
	await expect(first).toHaveAttribute('aria-pressed', 'true');
	await first.click();
	await expect(page.getByText('0 of 3 images selected')).toBeVisible();
	await first.click();
	await last.click();
	await expect(page.getByText('2 of 3 images selected')).toBeVisible();
	await expect(middle).toHaveAttribute('aria-pressed', 'false');

	await expect(start).toBeEnabled();
	await start.click();
	await expect.poll(() => uploads.length).toBe(2);
	expect(uploads.some((body) => body.includes(imageUrls[0]))).toBe(true);
	expect(uploads.some((body) => body.includes(imageUrls[1]))).toBe(false);
	expect(uploads.some((body) => body.includes(imageUrls[2]))).toBe(true);
	await expect(page.getByText('2 uploaded')).toBeVisible();
});
