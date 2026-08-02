import { test, expect, type Page } from '@playwright/test';

// Very tall images ("long pins") get two special treatments, both easy to regress:
//   - grid tiles are clamped to GRID_MAX_ASPECT and top-cropped, so one strip can't
//     stretch a masonry column past the infinite-scroll sentinel;
//   - the mobile detail view pins the Save pill to the bottom of the viewport while
//     the image overflows, then lets it land once the image ends.

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

function makeSave(i: number, width: number, height: number) {
	return {
		uri: `at://did:plc:test/is.currents.save/s${i}`,
		author: { did: me.did, handle: me.handle, displayName: me.displayName },
		content: {
			$type: 'is.currents.content.defs#imageView',
			blobCid: `bafy${i}`,
			imageUrl: `${APPVIEW}/img/${me.did}/bafy${i}?w=${width}&h=${height}`,
			width,
			height
		},
		originUrl: 'https://example.com/pin',
		createdAt: '2026-01-01T00:00:00Z',
		viewer: { saves: [] }
	};
}

// A 1:8 strip followed by an ordinary portrait.
const feed = [makeSave(1, 400, 3200), makeSave(2, 400, 520)];

async function mockApi(page: Page) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/img/')) {
			const params = new URL(url).searchParams;
			const w = params.get('w') ?? '400';
			const h = params.get('h') ?? '500';
			return route.fulfill({
				status: 200,
				contentType: 'image/svg+xml',
				body: `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}"><rect width="100%" height="100%" fill="#3b82f6"/></svg>`
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
		return json({});
	});
}

test('a 1:8 image is clamped to a 1:2 tile in the grid', async ({ page }) => {
	await mockApi(page);
	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });

	const box = await page.locator('a.block img').first().boundingBox();
	expect(box).not.toBeNull();
	// Unclamped this would be 8× the width; allow a hair for rounding.
	expect(box!.height / box!.width).toBeLessThanOrEqual(2.05);
});

test('the mobile Save pill stays on screen while a long image overflows, then lands', async ({
	page
}) => {
	await mockApi(page);
	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });
	await page.locator('a.block').first().click();

	const overlay = page.locator('.fixed.inset-0.z-50').first();
	await expect(overlay).toBeVisible();
	const save = page.getByRole('button', { name: /^Save$/ }).last();
	const viewport = page.viewportSize()!.height;

	// Pinned: the image runs way past the fold, but Save sits at the bottom of it.
	const pinned = (await save.boundingBox())!;
	expect(pinned.y + pinned.height).toBeLessThanOrEqual(viewport);
	expect(pinned.y + pinned.height).toBeGreaterThan(viewport - 120);

	// Landed: at the end it sits below the details instead of floating over them.
	// (`.last()`: the desktop hero renders the same markup behind `md:hidden`.)
	await overlay.evaluate((el) => el.scrollTo(0, el.scrollHeight));
	await expect(page.getByText('Source:').last()).toBeVisible();
	const accordion = (await page
		.getByRole('button', { name: 'In other collections' })
		.last()
		.boundingBox())!;
	const landed = (await save.boundingBox())!;
	expect(landed.y).toBeGreaterThanOrEqual(accordion.y + accordion.height);
});
