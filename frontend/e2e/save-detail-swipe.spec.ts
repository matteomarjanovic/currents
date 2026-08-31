import { test, expect, type Page } from '@playwright/test';

// Swiping the image in the detail view moves through the grid it was opened from.
// The grid is still mounted under the pushState overlay, so this is a state change,
// not a navigation — which is also why back has to land on the grid however many
// images were swiped through.

const APPVIEW = 'https://api-dev.currents.is';
// Match whichever appview PUBLIC_APPVIEW_URL currently names, not just the one the
// fixtures are written against — .env.development gets pointed at production for
// hands-on testing, and a mock that only knows the dev host silently intercepts
// nothing and leaves every request to fail on CORS.
const APPVIEW_ROUTE = /^https:\/\/api(-dev)?\.currents\.is\//;
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };

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
		createdAt: '2026-01-01T00:00:00Z',
		viewer: { saves: [] }
	};
}

const feed = [makeSave(1), makeSave(2), makeSave(3)];
const secondPage = [makeSave(4), makeSave(5), makeSave(6)];

// `paginate` gives the explore feed a second page, so a swipe run can outgrow what the
// grid had loaded when the tile was tapped. Without it the feed is a single page and the
// end of the run is the real end.
//
// `hydrate` serves feed items without viewer state, which is what makes the detail view
// fetch the open save again to fill it in. That second, later object is the thing that
// used to race the swipe, so any test about the swap needs it.
async function mockApi(page: Page, { paginate = false, hydrate = false } = {}) {
	const asFeedItem = (s: ReturnType<typeof makeSave>) =>
		hydrate ? { ...s, viewer: undefined } : s;
	await page.route(APPVIEW_ROUTE, (route) => {
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
		if (url.includes('getActorCollections')) return json({ collections: [] });
		if (url.includes('getFeed')) {
			if (!paginate) return json({ feed: feed.map(asFeedItem), cursor: null });
			const cursor = new URL(url).searchParams.get('cursor');
			return cursor
				? json({ feed: secondPage.map(asFeedItem), cursor: null })
				: json({ feed: feed.map(asFeedItem), cursor: 'page-2' });
		}
		if (url.includes('getSaves')) {
			const uri = decodeURIComponent(new URL(url).searchParams.get('uris') ?? '');
			return json({ saves: [...feed, ...secondPage].filter((s) => s.uri === uri) });
		}
		if (url.includes('getRelatedSaves')) return json({ saves: [] });
		if (url.includes('getImageCollections')) return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

// The gesture is direction-locked off the first decisive movement, so a swipe has to
// arrive as a sequence of moves rather than one jump — dispatch it through CDP.
async function drag(page: Page, dx: number, dy: number) {
	// The desktop pane renders first and is display:none at this viewport, so take the
	// visible image — the one on the mobile stage that carries the gesture.
	const stage = page.locator('.fixed.inset-0.z-50 img:visible').first();
	const box = (await stage.boundingBox())!;
	const from = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
	const client = await page.context().newCDPSession(page);
	await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [from] });
	for (let i = 1; i <= 5; i++) {
		await client.send('Input.dispatchTouchEvent', {
			type: 'touchMove',
			touchPoints: [{ x: from.x + (dx * i) / 5, y: from.y + (dy * i) / 5 }]
		});
	}
	await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
}

async function pinch(
	page: Page,
	target: ReturnType<Page['locator']>,
	fromDistance: number,
	toDistance: number,
	opensFocus = false
) {
	const box = (await target.boundingBox())!;
	const center = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
	const client = await page.context().newCDPSession(page);
	const points = (distance: number) => [
		{ id: 1, x: center.x - distance / 2, y: center.y },
		{ id: 2, x: center.x + distance / 2, y: center.y }
	];

	await client.send('Input.dispatchTouchEvent', {
		type: 'touchStart',
		touchPoints: points(fromDistance)
	});
	if (opensFocus) await expect(page.locator('[data-image-focus]')).toBeVisible();
	for (let i = 1; i <= 5; i++) {
		const distance = fromDistance + ((toDistance - fromDistance) * i) / 5;
		await client.send('Input.dispatchTouchEvent', {
			type: 'touchMove',
			touchPoints: points(distance)
		});
	}
	await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
}

async function emulateNativeApp(page: Page) {
	// Capacitor detects Android from this bridge before its client module initializes.
	await page.addInitScript(() => {
		Object.defineProperty(window, 'androidBridge', { value: {}, configurable: true });
	});
}

async function focusedScale(page: Page) {
	const style = await page
		.locator('[data-image-focus] > [role="presentation"]')
		.getAttribute('style');
	return Number(style?.match(/scale\(([^)]+)\)/)?.[1] ?? 0);
}

async function expectSaveReady(page: Page, rkey: string) {
	await expect(page).toHaveURL(new RegExp(`/save/${rkey}$`));
	// replaceState lands one frame before the swipe lock is released. Waiting for both
	// mirrors a person lifting and setting their finger down again instead of asking CDP
	// to start the next gesture inside that otherwise unobservable frame.
	await expect(page.locator('[data-swipe-stage]')).toHaveAttribute('data-swiping', 'false');
}

async function openFirstSave(page: Page) {
	await mockApi(page);
	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });
	await page.locator('a.block:has(img[src*="bafy1"])').tap();
	await expect(page).toHaveURL(/\/save\/s1$/);
}

test('swiping moves to the next and previous image of the grid', async ({ page }) => {
	await openFirstSave(page);

	await drag(page, -140, 0);
	await expectSaveReady(page, 's2');

	await drag(page, -140, 0);
	await expectSaveReady(page, 's3');

	await drag(page, 140, 0);
	await expectSaveReady(page, 's2');
});

test('swiping past the end of the grid stays put', async ({ page }) => {
	await openFirstSave(page);

	await drag(page, 140, 0);
	await page.waitForTimeout(500);
	await expect(page).toHaveURL(/\/save\/s1$/);
});

test('a vertical drag scrolls instead of changing the image', async ({ page }) => {
	await openFirstSave(page);

	await drag(page, 0, -200);
	await page.waitForTimeout(500);
	await expect(page).toHaveURL(/\/save\/s1$/);
});

test('web pinch stays with the browser instead of opening image focus', async ({ page }) => {
	await openFirstSave(page);

	await pinch(page, page.getByRole('button', { name: 'View image full screen' }), 80, 240);
	await expect(page.locator('[data-image-focus]')).toHaveCount(0);
});

test('mobile web exposes the image actions drawer', async ({ page }) => {
	await openFirstSave(page);

	await page.getByRole('button', { name: 'Image actions' }).tap();
	await expect(page.getByText('Image actions', { exact: true })).toBeVisible();
	for (const action of ['Download', 'Copy image', 'Share', 'Copy link', 'Hide', 'Report']) {
		await expect(page.getByRole('button', { name: action, exact: true })).toBeVisible();
	}
});

test('native pinch opens focus and continuously zooms on the first gesture', async ({ page }) => {
	await emulateNativeApp(page);
	await openFirstSave(page);

	await pinch(page, page.getByRole('button', { name: 'View image full screen' }), 80, 240, true);
	await expect.poll(() => focusedScale(page)).toBeGreaterThan(2.5);
	await expect(page.getByRole('button', { name: 'Close image viewer' })).toBeVisible();

	await page.getByRole('button', { name: 'Close image viewer' }).tap();
	await expect(page.locator('[data-image-focus]')).toHaveCount(0);
	await drag(page, -140, 0);
	await expectSaveReady(page, 's2');
});

test('tapping the image opens a dimmed viewer that pinches to zoom in and out', async ({
	page
}) => {
	await openFirstSave(page);

	await page.getByRole('button', { name: 'View image full screen' }).tap();
	const viewer = page.locator('[data-image-focus]');
	await expect(viewer).toBeVisible();
	await expect(viewer.locator('img')).toHaveAttribute('src', /bafy1/);
	await expect(page.getByRole('dialog')).toHaveClass(/bg-black\/85/);

	await pinch(page, viewer, 80, 240);
	await expect.poll(() => focusedScale(page)).toBeGreaterThan(2.5);

	await pinch(page, viewer, 240, 40);
	await expect.poll(() => focusedScale(page)).toBe(1);

	await page.getByRole('button', { name: 'Close image viewer' }).tap();
	await expect(viewer).toHaveCount(0);

	// Closing focus hands the stage straight back to its existing sequence gesture.
	await drag(page, -140, 0);
	await expectSaveReady(page, 's2');
});

test('the neighbours are rendered either side, loaded and ready to slide in', async ({ page }) => {
	await openFirstSave(page);
	const overlay = page.locator('.fixed.inset-0.z-50');

	// s1's next neighbour is on the stage from the outset — off to the side and clipped,
	// but in the DOM and loading eagerly, which is what makes a drag reveal a picture
	// rather than an empty box. Two images away is not.
	await expect(overlay.locator('img[src*="bafy2"]')).toHaveAttribute('loading', 'eager');
	await expect(overlay.locator('img[src*="bafy3"]')).toHaveCount(0);

	await drag(page, -140, 0);
	await expectSaveReady(page, 's2');

	// …and from the middle of the run, both sides are there.
	await expect(overlay.locator('img[src*="bafy1"]')).toHaveCount(1);
	await expect(overlay.locator('img[src*="bafy3"]')).toHaveCount(1);
});

test('the run grows past the page the grid had loaded when the tile was tapped', async ({
	page
}) => {
	await mockApi(page, { paginate: true });
	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });
	await page.locator('a.block:has(img[src*="bafy1"])').tap();
	// Let the detail settle before dragging: the stage exists a beat before it knows what
	// its neighbours are, and a drag landing in that gap is simply ignored.
	await expect(page).toHaveURL(/\/save\/s1$/);

	// s3 ends the page the grid was holding. The grid's own sentinel can't notice — it's
	// under the overlay — so the detail has to have asked for the next page itself.
	await drag(page, -140, 0);
	await expectSaveReady(page, 's2');
	await drag(page, -140, 0);
	await expectSaveReady(page, 's3');

	await drag(page, -140, 0);
	await expectSaveReady(page, 's4');
	await drag(page, -140, 0);
	await expectSaveReady(page, 's5');
});

test('hydrating the open save neither refetches its rail nor repaints the previous image', async ({
	page
}) => {
	const asked: string[] = [];
	await mockApi(page, { hydrate: true });
	page.on('request', (r) => {
		const u = r.url();
		if (u.includes('getRelatedSaves')) {
			asked.push(
				decodeURIComponent(new URL(u).searchParams.get('uri') ?? '')
					.split('/')
					.pop()!
			);
		}
	});

	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });
	await page.locator('a.block:has(img[src*="bafy1"])').tap();
	await expect(page).toHaveURL(/\/save\/s1$/);

	// Hydration lands a second save object for s1. Keyed on the object rather than the
	// uri, everything downstream re-ran — a second rail fetch for an image already shown,
	// and, for a frame, the old picture painted back over a just-completed swipe.
	await expect.poll(() => asked).toEqual(['s1']);

	await drag(page, -140, 0);
	await expectSaveReady(page, 's2');
	await expect.poll(() => asked).toEqual(['s1', 's2']);

	// The swap is what the viewer sees: the centre pane has to be the image the URL says.
	await expect(page.locator('.fixed.inset-0.z-50 img:visible').first()).toHaveAttribute(
		'src',
		/bafy2/
	);
});

test('the related rail opens with a small page and keeps full pages after', async ({ page }) => {
	const limits: string[] = [];
	await mockApi(page);
	page.on('request', (r) => {
		if (r.url().includes('getRelatedSaves')) {
			limits.push(new URL(r.url()).searchParams.get('limit') ?? '');
		}
	});

	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });
	await page.locator('a.block:has(img[src*="bafy1"])').tap();

	// The opening fetch is paid again on every swipe, so it stays small; the sentinel
	// asks for full pages once the viewer actually scrolls into the rail.
	await expect.poll(() => limits).toEqual(['12']);
});

test('back returns to the grid however many images were swiped through', async ({ page }) => {
	await openFirstSave(page);

	await drag(page, -140, 0);
	await expectSaveReady(page, 's2');
	await drag(page, -140, 0);
	await expectSaveReady(page, 's3');

	await page.goBack();
	await expect(page).toHaveURL(/\/explore\/general$/);
	await expect(page.locator('.fixed.inset-0.z-50')).toHaveCount(0);
});
