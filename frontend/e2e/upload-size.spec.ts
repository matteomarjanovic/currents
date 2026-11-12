import { expect, test } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const LIMIT = 20_000_000;
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };

for (const size of [LIMIT, LIMIT + 1]) {
	test(`upload ${size} bytes respects the image limit`, async ({ page }) => {
		let tokenRequests = 0;
		let uploads = 0;
		let saves = 0;
		await page.route(`${APPVIEW}/**`, (route) => {
			const url = route.request().url();
			const json = (body: unknown) =>
				route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
			if (url.endsWith('/api/me')) return json(me);
			if (url.includes('getActorCollections')) return json({ collections: [] });
			if (url.includes('features/seen')) return json({ seen: [] });
			if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
			if (url.includes('/api/blob/upload-token')) {
				tokenRequests++;
				return json({ token: 'test-token', pdsUrl: 'https://pds.test' });
			}
			if (url.endsWith('/api/save')) {
				saves++;
				return json({ uri: 'at://did:plc:test/is.currents.feed.save/new' });
			}
			return json({});
		});
		await page.route('https://pds.test/**', (route) => {
			uploads++;
			expect(route.request().postDataBuffer()?.length).toBe(size);
			return route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					blob: { $type: 'blob', ref: { $link: 'test' }, size, mimeType: 'image/gif' }
				})
			});
		});
		await page.goto('/upload');
		await page.getByRole('button', { name: 'Select collection' }).click();
		await page.getByRole('button', { name: /Profile.*Save without a collection/ }).click();
		const buffer = Buffer.alloc(size);
		Buffer.from('R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7', 'base64').copy(buffer);
		await page
			.locator('input[type=file]')
			.first()
			.setInputFiles({ name: 'test.gif', mimeType: 'image/gif', buffer });
		await page.getByRole('button', { name: 'Start upload', exact: true }).click();
		if (size > LIMIT) {
			await expect(page.getByRole('alert')).toHaveText('Images must be 20 MB or smaller.');
			expect({ tokenRequests, uploads, saves }).toEqual({ tokenRequests: 0, uploads: 0, saves: 0 });
		} else {
			await expect(page.getByText('1 uploaded', { exact: true })).toBeVisible();
			expect({ tokenRequests, uploads, saves }).toEqual({ tokenRequests: 1, uploads: 1, saves: 1 });
		}
	});
}
