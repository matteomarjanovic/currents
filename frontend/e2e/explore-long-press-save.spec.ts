import { test, expect, type Page } from '@playwright/test';

// Explore is the one grid where a tile is a link to the detail view, so a normal
// tap has to keep navigating — long-pressing is the only way to reach the
// collection drawer without a trip through the detail view first.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const collections = [
	{
		uri: 'at://did:plc:test/is.currents.collection/c1',
		author: { did: me.did, handle: me.handle, displayName: me.displayName },
		name: 'Test Collection',
		previews: [],
		saveCount: 1,
		createdAt: '2026-01-01T00:00:00Z'
	}
];

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

	await page.getByRole('button', { name: 'Test Collection' }).click();
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
