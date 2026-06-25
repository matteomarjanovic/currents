import { test, expect, type Page } from '@playwright/test';

// Regression test for the nested scroll-lock leak: opening the save drawer over an image
// (which locks body scroll) and then returning to the grid used to leave the page stuck
// unscrollable, because vaul-svelte snapshots/restores `document.body.style.cssText` and
// re-applied the locked inline styles after close. The lock now lives in a body class
// (see src/lib/scroll-lock.ts), which vaul's cssText restore can't touch.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const collections = [
	{
		uri: 'at://did:plc:test/is.currents.collection/c1',
		author: { did: me.did, handle: me.handle, displayName: me.displayName },
		name: 'Test Collection',
		previews: [],
		saveCount: 2,
		createdAt: '2026-01-01T00:00:00Z'
	}
];
const feed = Array.from({ length: 10 }, (_, i) => ({
	uri: `at://did:plc:test/is.currents.save/s${i + 1}`,
	author: { did: me.did, handle: me.handle, displayName: me.displayName },
	content: {
		$type: 'is.currents.content.defs#imageView',
		blobCid: `bafy${i + 1}`,
		imageUrl: `${APPVIEW}/img/${me.did}/bafy${i + 1}`,
		width: 400,
		height: 460 + i * 25
	},
	createdAt: '2026-01-01T00:00:00Z',
	viewer: { saves: [] }
}));

async function mockApi(page: Page) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/img/'))
			return route.fulfill({
				status: 200,
				contentType: 'image/svg+xml',
				body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"><rect width="100%" height="100%" fill="#ccc"/></svg>'
			});
		if (url.includes('/api/me')) return json(me);
		if (url.includes('getActorCollections')) return json({ collections });
		if (url.includes('getFeed')) return json({ feed, cursor: null });
		if (url.includes('getSaves')) return json({ saves: [feed[0]] });
		if (url.includes('getRelatedSaves')) return json({ saves: [] });
		if (url.includes('getImageCollections')) return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		if (url.includes('/resave') || url.match(/\/save(\/|\b)/))
			return json({ uri: 'at://did:plc:test/is.currents.save/new1' });
		return json({});
	});
}

test('save drawer over an image leaves the page scrollable after closing', async ({ page }) => {
	await mockApi(page);
	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });

	for (let i = 0; i < 3; i++) {
		// Open the image overlay (locks background scroll).
		await page.locator('a.block').first().click();
		await expect(page.locator('.fixed.inset-0.z-50').first()).toBeVisible();

		// Open the save drawer (vaul snapshots body styles here).
		await page
			.getByRole('button', { name: /^Save$/ })
			.last()
			.click();
		await expect(page.locator('[data-vaul-drawer]')).toBeVisible();

		// Swipe the drawer down to dismiss (best effort), then return to the grid.
		const box = await page.locator('[data-vaul-drawer]').boundingBox();
		if (box) {
			const cx = box.x + box.width / 2;
			await page.mouse.move(cx, box.y + 24);
			await page.mouse.down();
			for (let y = 30; y <= 360; y += 30) await page.mouse.move(cx, box.y + 24 + y, { steps: 2 });
			await page.mouse.up();
		}
		await page.goBack();

		// Wait past vaul's delayed body cssText restore (~300ms) before asserting.
		await page.waitForTimeout(600);
		const locked = await page.evaluate(() => {
			const s = getComputedStyle(document.body);
			return s.position === 'fixed' || s.overflow === 'hidden';
		});
		expect(locked, `body left scroll-locked on iteration ${i}`).toBe(false);
	}
});
