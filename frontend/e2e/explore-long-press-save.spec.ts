import { test, expect, type Page } from '@playwright/test';

// Explore is the one grid where a tile is a link to the detail view, so a normal
// tap has to keep navigating — long-pressing is the only way to reach the
// collection drawer without a trip through the detail view first.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const collections = Array.from({ length: 30 }, (_, i) => ({
	uri: `at://did:plc:test/is.currents.collection/c${i}`,
	author: { did: me.did, handle: me.handle, displayName: me.displayName },
	name: i === 0 ? 'Test Collection' : `Test Collection ${i + 1}`,
	previews: [],
	saveCount: 1,
	createdAt: '2026-01-01T00:00:00Z'
}));

function makeSave(i: number) {
	return {
		uri: `at://did:plc:test/is.currents.save/s${i}`,
		author: { did: me.did, handle: me.handle, displayName: me.displayName },
		content: {
			$type: 'is.currents.content.defs#imageView',
			blobCid: `bafy${i}`,
			imageUrl: `${APPVIEW}/img/${me.did}/bafy${i}?w=400&h=500`,
			width: 400,
			height: 500
		},
		originUrl: 'https://example.com/pin',
		createdAt: '2026-01-01T00:00:00Z',
		viewer: { saves: [] }
	};
}

const feed = [makeSave(1)];

async function mockApi(page: Page) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/img/')) {
			return route.fulfill({
				status: 200,
				contentType: 'image/svg+xml',
				body: `<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"><rect width="100%" height="100%" fill="#3b82f6"/></svg>`
			});
		}
		if (url.includes('/api/me')) return json(me);
		if (url.includes('getActorCollections')) return json({ collections });
		if (url.includes('getFeed')) return json({ feed, cursor: null });
		if (url.includes('getSaves')) return json({ saves: [feed[0]] });
		if (url.includes('getRelatedSaves')) return json({ saves: [] });
		if (url.includes('getImageCollections')) return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		if (url.includes('/resave')) return json({ uri: 'at://did:plc:test/is.currents.save/new' });
		return json({});
	});
}

// Playwright's `.tap()` fires pointerdown/up back-to-back with no gap, so a real
// long-press needs a held touch — drive it through CDP directly.
async function longPress(page: Page, locator: ReturnType<Page['locator']>, ms = 650) {
	const box = (await locator.boundingBox())!;
	const point = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
	const client = await page.context().newCDPSession(page);
	await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [point] });
	await page.waitForTimeout(ms);
	await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
}

async function touchDrag(page: Page, point: { x: number; y: number }, distance: number) {
	const client = await page.context().newCDPSession(page);
	await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [point] });
	for (let step = 1; step <= 12; step++) {
		await client.send('Input.dispatchTouchEvent', {
			type: 'touchMove',
			touchPoints: [{ x: point.x, y: point.y + (distance * step) / 12 }]
		});
	}
	await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
}

test('long-pressing a tile opens the collection drawer and saves without navigating', async ({
	page
}) => {
	await mockApi(page);
	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });

	// Pressing right away, with no settle delay, used to race explore's
	// debounced feed-reload effect (fired once, unconditionally, ~300ms after
	// every mount) and lose the gesture when it tore the tile down mid-press.
	await longPress(page, page.locator('a.block').first());

	await expect(page.getByText('Save to collection')).toBeVisible();
	await expect(page).toHaveURL(/\/explore\/general/);

	await page.getByRole('button', { name: 'Test Collection Public', exact: true }).click();
	await expect(page.getByText('Save to collection')).toBeHidden();
});

test('a short tap still opens the detail view instead of the drawer', async ({ page }) => {
	await mockApi(page);
	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });

	await page.locator('a.block').first().tap();

	await expect(page.locator('.fixed.inset-0.z-50').first()).toBeVisible();
	await expect(page.getByText('Save to collection')).not.toBeVisible();
});

test('a scrolled list must be released before a collection row can close the drawer', async ({
	page
}) => {
	await mockApi(page);
	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });
	await longPress(page, page.locator('a.block').first());
	await page.waitForTimeout(600); // Vaul's opening animation is intentionally not draggable.

	const list = page.locator('[data-vaul-drawer] > div.overflow-y-auto');
	const box = (await list.boundingBox())!;
	const row = { x: box.x + box.width / 2, y: box.y + 30 };
	await touchDrag(page, row, -180); // Scroll the list down.
	await expect.poll(() => list.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
	// The gesture that returns the list to its top must not also drag the drawer.
	await touchDrag(page, row, 420);
	await expect(page.getByText('Save to collection')).toBeVisible();
	await expect.poll(() => list.evaluate((element) => element.scrollTop)).toBe(0);

	// A fresh gesture that begins at the top owns the drawer.
	await touchDrag(page, row, 260);

	await expect(page.getByText('Save to collection')).toBeHidden();
});
