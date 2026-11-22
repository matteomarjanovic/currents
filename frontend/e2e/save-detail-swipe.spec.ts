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
const APPVIEW_ROUTE = /^https:\/\/(?:api(?:-dev)?\.)?currents\.is\//;
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
async function mockApi(page: Page, { paginate = false, hydrate = false, related = false } = {}) {
	const asFeedItem = (s: ReturnType<typeof makeSave>) =>
		hydrate ? { ...s, viewer: undefined } : s;
	await page.route(APPVIEW_ROUTE, async (route) => {
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
		if (url.includes('getRelatedSaves')) {
			if (!related) return json({ saves: [] });
			const cursor = new URL(url).searchParams.get('cursor');
			if (!cursor) {
				return json({
					saves: Array.from({ length: 20 }, (_, i) => makeSave(i + 10)),
					cursor: 'page-2'
				});
			}
			await new Promise((resolve) => setTimeout(resolve, 250));
			const start = cursor === 'page-2' ? 30 : cursor === 'page-3' ? 80 : 130;
			return json({
				saves: Array.from({ length: 50 }, (_, i) => makeSave(i + start)),
				cursor: cursor === 'page-2' ? 'page-3' : cursor === 'page-3' ? 'page-4' : null
			});
		}
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
		Object.assign(window, {
			Capacitor: {
				PluginHeaders: [
					{ name: 'SecurePreferences', methods: [{ name: 'get', rtype: 'promise' }] }
				],
				async nativePromise() {
					return { value: null };
				}
			}
		});
	});
}

async function emulateIosApp(page: Page) {
	// Capacitor detects iOS from this bridge before its client module initializes.
	await page.addInitScript(() => {
		Object.defineProperty(window, 'webkit', {
			value: { messageHandlers: { bridge: { postMessage() {} } } },
			configurable: true
		});
		Object.assign(window, {
			Capacitor: {
				PluginHeaders: [
					{ name: 'SecurePreferences', methods: [{ name: 'get', rtype: 'promise' }] }
				],
				async nativePromise() {
					return { value: null };
				}
			}
		});
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

test('desktop related scrolling shows a larger centered Go to top without a floating Back button', async ({
	page
}) => {
	await page.setViewportSize({ width: 1440, height: 1000 });
	await mockApi(page, { related: true });
	await page.goto('/explore/general');
	await page.locator('a.block:has(img[src*="bafy1"])').click();
	const overlay = page.locator('[data-save-detail-overlay]');
	await overlay.evaluate((element) => element.scrollTo(0, 1100));
	const button = overlay.getByRole('button', { name: 'Go to top', exact: true });
	await expect(button).toBeVisible();
	await expect(button).toHaveCSS('height', '44px');
	await expect
		.poll(async () => {
			const box = (await button.boundingBox())!;
			return Math.abs(box.x + box.width / 2 - 720);
		})
		.toBeLessThan(1);
	expect(
		await overlay.getByRole('button', { name: 'Go back', exact: true }).evaluateAll((buttons) =>
			buttons.some((button) => {
				const box = button.getBoundingClientRect();
				return box.width > 0 && box.height > 0 && box.top >= 0 && box.bottom <= innerHeight;
			})
		)
	).toBe(false);
	await button.click();
	await expect.poll(() => overlay.evaluate((element) => element.scrollTop)).toBe(0);
});

test('swiping moves to the next and previous image of the grid', async ({ page }) => {
	await openFirstSave(page);

	await drag(page, -140, 0);
	await expectSaveReady(page, 's2');

	await drag(page, -140, 0);
	await expectSaveReady(page, 's3');

	await drag(page, 140, 0);
	await expectSaveReady(page, 's2');
});

test('desktop arrow keys navigate the grid sequence and Back returns to Explore', async ({
	page
}) => {
	await page.setViewportSize({ width: 1440, height: 1000 });
	await mockApi(page, { paginate: true });
	await page.goto('/explore/general');
	await page.locator('a.block:has(img[src*="bafy1"])').click();
	await expect(page).toHaveURL(/\/save\/s1$/);
	await page.keyboard.press('ArrowLeft');
	await expect(page).toHaveURL(/\/save\/s1$/);
	for (const rkey of ['s2', 's3', 's4']) {
		await page.keyboard.press('ArrowRight');
		await expect(page).toHaveURL(new RegExp(`/save/${rkey}$`));
	}
	await page.keyboard.press('ArrowLeft');
	await expect(page).toHaveURL(/\/save\/s3$/);
	await expect(page.locator('[data-save-detail-overlay] img:visible').first()).toHaveAttribute(
		'src',
		/bafy3/
	);
	await page.goBack();
	await expect(page).toHaveURL(/\/explore\/general$/);
	await expect(page.locator('[data-save-detail-overlay]')).toHaveCount(0);
});

test('desktop arrow keys leave editing, dialogs, and modified shortcuts alone', async ({
	page
}) => {
	await page.setViewportSize({ width: 1440, height: 1000 });
	await openFirstSave(page);
	await page.keyboard.press('Alt+ArrowRight');
	await expect(page).toHaveURL(/\/save\/s1$/);
	for (const tag of ['input', 'textarea', 'select', 'div']) {
		await page.locator('[data-save-detail-overlay]').evaluate((overlay, tag) => {
			const editor = document.createElement(tag);
			editor.id = 'keyboard-test-editor';
			if (tag === 'div') editor.contentEditable = 'true';
			overlay.append(editor);
			editor.focus();
		}, tag);
		await page.keyboard.press('ArrowRight');
		await expect(page).toHaveURL(/\/save\/s1$/);
		await page.locator('#keyboard-test-editor').evaluate((editor) => editor.remove());
	}
	await page
		.locator('[data-save-detail-overlay]')
		.getByRole('button', { name: 'Image actions' })
		.filter({ visible: true })
		.click();
	await expect(page.getByRole('menu')).toBeVisible();
	await page.keyboard.press('ArrowRight');
	await expect(page).toHaveURL(/\/save\/s1$/);
	await page.getByRole('menuitem', { name: 'Report', exact: true }).click();
	await expect(page.getByRole('dialog', { name: 'Report this image' })).toBeVisible();
	await page.getByRole('button', { name: 'Cancel', exact: true }).focus();
	await page.keyboard.press('ArrowRight');
	await expect(page).toHaveURL(/\/save\/s1$/);
	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog')).toHaveCount(0);
	await page.keyboard.press('ArrowRight');
	await expect(page).toHaveURL(/\/save\/s2$/);
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

test('related thumbnails load before scrolling without fetching the next page', async ({
	page
}) => {
	await page.setViewportSize({ width: 1440, height: 1000 });
	await mockApi(page, { related: true });
	const thumbnails = new Set<string>();
	const limits: string[] = [];
	page.on('request', (request) => {
		const url = new URL(request.url());
		const cid = url.pathname.match(/\/bafy(\d+)$/)?.[1];
		if (cid && Number(cid) >= 10) thumbnails.add(cid);
		if (url.pathname.includes('getRelatedSaves')) limits.push(url.searchParams.get('limit') ?? '');
	});
	await page.goto('/explore/general');
	await page.locator('a.block:has(img[src*="bafy1"])').click();
	await expect(page).toHaveURL(/\/save\/s1$/);
	const overlay = page.locator('[data-save-detail-overlay]');
	const lastThumbnail = overlay.locator('section img[src*="bafy29"]');
	await expect(lastThumbnail).toHaveAttribute('loading', 'eager');
	await expect.poll(() => thumbnails.size).toBe(20);
	expect(
		await lastThumbnail.evaluate((image) => image.getBoundingClientRect().top)
	).toBeGreaterThan(1000);
	expect(await overlay.evaluate((element) => element.scrollTop)).toBe(0);
	expect(limits).toEqual(['20']);
});

test('the related rail opens with a small page and keeps full pages after', async ({ page }) => {
	const limits: string[] = [];
	await mockApi(page, { related: true });
	page.on('request', (r) => {
		if (r.url().includes('getRelatedSaves')) {
			limits.push(new URL(r.url()).searchParams.get('limit') ?? '');
		}
	});

	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });
	await page.locator('a.block:has(img[src*="bafy1"])').tap();

	// The opening fetch is paid again on every swipe, so it stays small. The first
	// detail scroll immediately asks for a full page, before the rail is exhausted.
	await expect.poll(() => limits).toEqual(['20']);
	await page.locator('[data-save-detail-overlay]').evaluate((element) => {
		element.scrollTop = 1;
		element.dispatchEvent(new Event('scroll'));
	});
	await expect.poll(() => limits).toEqual(['20', '50']);
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

test('iOS back from a detail preserves the feed scroll position', async ({ page }) => {
	await emulateIosApp(page);
	await mockApi(page);
	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });
	await page.locator('main').evaluate((element) => (element.style.minHeight = '2000px'));
	await page.evaluate(() => window.scrollTo(0, 300));

	await page.locator('a.block:has(img[src*="bafy3"])').tap();
	await expect(page).toHaveURL(/\/save\/s3$/);
	await expect(page.locator('body')).toHaveCSS('top', '-300px');
	await page.goBack();
	await expect(page).toHaveURL(/\/explore\/general$/);
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(300);
});

for (const [platform, emulate] of [
	['iOS', emulateIosApp],
	['Android', emulateNativeApp]
] as const) {
	test(`${platform} keeps the previous detail mounted under a related image`, async ({ page }) => {
		await emulate(page);
		await mockApi(page, { related: true });
		let feedRequests = 0;
		const relatedRequests = new Map<string, number>();
		page.on('request', (request) => {
			if (request.url().includes('getFeed')) feedRequests += 1;
			if (request.url().includes('getRelatedSaves')) {
				const uri = new URL(request.url()).searchParams.get('uri') ?? '';
				relatedRequests.set(uri, (relatedRequests.get(uri) ?? 0) + 1);
			}
		});

		await page.goto('/explore/general');
		await page.waitForSelector('a.block img', { timeout: 10_000 });
		await page.locator('main').evaluate((element) => (element.style.minHeight = '2000px'));
		await page.evaluate(() => window.scrollTo(0, 300));
		const feedScroll = await page.evaluate(() => window.scrollY);
		expect(feedScroll).toBeGreaterThan(0);

		await page.locator('a.block:has(img[src*="bafy3"])').tap();
		const overlays = page.locator('[data-save-detail-overlay]');
		await expect(overlays).toHaveCount(1);
		const overlay = overlays.first();
		const relatedGrid = overlay.locator('section div[style*="grid-template-columns"]').first();
		await expect
			.poll(() => relatedGrid.evaluate((element) => element.children.length))
			.toBeGreaterThanOrEqual(21);
		for (const frameCount of [71, 121, 171]) {
			await overlay.evaluate((element) => element.scrollTo(0, element.scrollHeight));
			await expect
				.poll(() => relatedGrid.evaluate((element) => element.children.length))
				.toBeGreaterThanOrEqual(frameCount);
		}
		await overlay.evaluate((element) => element.scrollTo(0, 16_000));
		await expect.poll(() => overlay.evaluate((element) => element.scrollTop)).toBe(16_000);
		const visibleHref = async () =>
			overlay.locator('section a.block').evaluateAll((links) => {
				const link = links.find((element) => {
					const rect = element.getBoundingClientRect();
					return rect.top >= 0 && rect.bottom <= window.innerHeight;
				});
				return link?.getAttribute('href') ?? null;
			});
		await expect.poll(visibleHref).not.toBeNull();
		const href = (await visibleHref())!;
		const detailScroll = await overlay.evaluate((element) => element.scrollTop);
		expect(detailScroll).toBe(16_000);
		const requestsBeforePush = relatedRequests.get(feed[2].uri);
		await overlay.locator(`a[href="${href}"]`).tap();
		await expect(page).toHaveURL(new RegExp(`${href.split('/').pop()}$`));
		await expect(overlays).toHaveCount(2);
		await expect(overlays.first()).toHaveAttribute('data-active', 'false');
		await expect(overlays.last()).toHaveAttribute('data-active', 'true');
		expect(await overlays.first().evaluate((element) => element.scrollTop)).toBe(detailScroll);

		// A third level suspends the first detail's heavy DOM, but retains its pages/state.
		await overlays.last().locator('section a.block').first().tap();
		await expect(overlays).toHaveCount(3);
		await expect(overlays.first()).toHaveAttribute('data-rendered', 'false');
		await expect(overlays.first().locator('section')).toHaveCount(0);
		await page.goBack();
		await expect(overlays).toHaveCount(2);
		await expect(overlays.first()).toHaveAttribute('data-rendered', 'true');
		await expect
			.poll(() => overlays.first().evaluate((element) => element.scrollTop))
			.toBe(detailScroll);
		await expect
			.poll(() => relatedGrid.evaluate((element) => element.children.length))
			.toBeGreaterThanOrEqual(171);

		await page.goBack();
		await expect(page).toHaveURL(/\/save\/s3$/);
		await expect(overlays).toHaveCount(1);
		await expect(overlays.first()).toHaveAttribute('data-active', 'true');
		expect(await overlays.first().evaluate((element) => element.scrollTop)).toBe(detailScroll);
		expect(relatedRequests.get(feed[2].uri)).toBe(requestsBeforePush);
		await overlays.first().evaluate((element) => element.scrollTo(0, 0));
		await drag(page, 140, 0);
		await expectSaveReady(page, 's2');

		await page.goBack();
		await expect(page).toHaveURL(/\/explore\/general$/);
		await expect(overlays).toHaveCount(0);
		await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(feedScroll);
		expect(feedRequests).toBe(1);
	});
}

test('mobile image trail opens above the scroll-to-top button and returns to an earlier image', async ({
	page
}) => {
	await mockApi(page, { related: true });
	await page.goto('/explore/general');
	await page.locator('a.block:has(img[src*="bafy1"])').first().tap();

	const active = page.locator('[data-save-detail-overlay][data-active="true"]');
	const trigger = page.getByRole('button', { name: 'Open image history' });
	await expect(active.locator('[data-save-detail-top-fade]')).toHaveCSS(
		'background-image',
		/^linear-gradient/
	);
	await active.evaluate((el) => el.scrollTo(0, 1100));
	await active.locator('section a.block').first().tap();
	await expect(page.locator('[data-save-detail-overlay]')).toHaveCount(2);

	await active.evaluate((el) => el.scrollTo(0, 1100));
	const toTop = active.getByRole('button', { name: 'Scroll to top', exact: true });
	await expect(toTop).toBeVisible();
	await expect(trigger).toBeVisible();
	await toTop.evaluate(async (el) => {
		await Promise.all(el.parentElement!.getAnimations().map((animation) => animation.finished));
	});
	const stackBox = (await trigger.boundingBox())!;
	const buttonBox = (await toTop.boundingBox())!;
	expect(stackBox.y + stackBox.height).toBeLessThan(buttonBox.y);
	expect(Math.abs(stackBox.x + stackBox.width - buttonBox.x - buttonBox.width)).toBeLessThan(1);
	// The collapsed thumbnails are drawn through a portal and the trigger is
	// transparent. Test the visible thumbnail's position and tap that exact spot.
	const cover = page.locator('[data-mobile-history-layer] button[aria-current="step"]');
	const coverBox = (await cover.boundingBox())!;
	const x = coverBox.x + coverBox.width / 2;
	const y = coverBox.y + coverBox.height / 2;
	expect(Math.abs(x - (stackBox.x + stackBox.width / 2))).toBeLessThan(1);
	expect(y).toBeGreaterThan(stackBox.y);
	expect(y).toBeLessThan(stackBox.y + stackBox.height);
	await page.touchscreen.tap(x, y);
	const history = page.getByRole('dialog', { name: 'Image history' });
	await expect(history).toBeVisible();
	const earlier = history.getByRole('button', { name: 'Return to image 1', exact: true });
	const current = history.getByRole('button', { name: 'Return to image 2', exact: true });
	await expect(current).toHaveAttribute('aria-current', 'step');
	await expect
		.poll(async () => (await current.boundingBox())!.y - (await earlier.boundingBox())!.y)
		.toBeGreaterThan(50);
	const close = history.getByRole('button', { name: 'Close image history' });
	const closeBox = (await close.boundingBox())!;
	expect(Math.abs(closeBox.y + closeBox.height - buttonBox.y - buttonBox.height)).toBeLessThan(1);
	await close.tap();
	await expect(history).toBeHidden();
	await trigger.tap();
	await earlier.tap();
	await expect(page).toHaveURL(/\/save\/s1$/);
	await expect(page.locator('[data-save-detail-overlay]')).toHaveCount(1);
});

test('desktop image trail follows nested details and restores an earlier scroll position', async ({
	page
}) => {
	await page.setViewportSize({ width: 1440, height: 1000 });
	await mockApi(page, { related: true });
	await page.goto('/explore/general');
	await page.locator('a.block:has(img[src*="bafy1"])').click();
	const active = page.locator('[data-save-detail-overlay][data-active="true"]');
	const navigation = page.locator('[data-save-detail-navigation]');
	const trail = navigation.getByRole('navigation', { name: 'Image trail' });
	await expect(trail).toHaveCount(0);
	const logo = navigation.getByRole('link', { name: 'Currents', exact: true });
	const mode = navigation.getByRole('tablist', { name: 'Currents mode' });
	const logoTop = (await logo.boundingBox())!.y;
	const modeTop = (await mode.boundingBox())!.y;
	await active.evaluate((el) => el.scrollTo(0, 1100));
	await expect(trail).toBeVisible();
	await expect.poll(async () => (await logo.boundingBox())!.y).toBe(logoTop);
	await expect.poll(async () => (await mode.boundingBox())!.y).toBe(modeTop);
	await expect(trail.getByRole('button')).toHaveCount(1);
	await trail.evaluate((node) => (node.dataset.persistentTrail = 'original'));
	await active.locator('section a.block').first().click();
	await expect(trail).toHaveAttribute('data-persistent-trail', 'original');
	await expect(page.locator('[data-save-detail-overlay]')).toHaveCount(2);
	const originalScroll = await page
		.locator('[data-save-detail-overlay]')
		.first()
		.evaluate((el) => el.scrollTop);
	await expect(trail.getByRole('button')).toHaveCount(1);
	await active.evaluate((el) => el.scrollTo(0, 1100));
	await expect(trail.getByRole('button')).toHaveCount(2);
	// Two photos get a little extra breathing room, plus 12px padding per side.
	await expect
		.poll(() =>
			trail.evaluate((node) => {
				const widths = [...node.querySelectorAll('button')].reduce(
					(sum, button) => sum + button.offsetWidth,
					0
				);
				return Math.abs(node.getBoundingClientRect().width - widths - 20 - 24);
			})
		)
		.toBeLessThan(1);
	await active.locator('section a.block').nth(1).click();
	await expect(page.locator('[data-save-detail-overlay]')).toHaveCount(3);
	await active.evaluate((el) => el.scrollTo(0, 1100));
	await expect(trail.getByRole('button')).toHaveCount(3);
	await page.screenshot({ path: '/tmp/currents-trail-stacked.png' });
	const trailWidth = (await trail.boundingBox())!.width;
	await trail.getByRole('button').last().hover();
	await expect(trail).toHaveClass(/open/);
	// The clip boundary stays ready throughout expansion, rather than revealing
	// thumbnails behind a slower width transition.
	expect((await trail.boundingBox())!.width).toBe(trailWidth);
	await expect(trail).toHaveCSS('transition-duration', '0s');
	await expect.poll(async () => (await trail.boundingBox())!.width).toBeGreaterThan(88);
	await page.screenshot({ path: '/tmp/currents-trail-expanded.png' });
	await trail.getByRole('button', { name: 'Return to image 1', exact: true }).click();
	await expect(page).toHaveURL(/\/save\/s1$/);
	await expect(page.locator('[data-save-detail-overlay]')).toHaveCount(1);
	await expect.poll(() => active.evaluate((el) => el.scrollTop)).toBe(originalScroll);
	await trail.getByRole('button').click();
	await expect.poll(() => active.evaluate((el) => el.scrollTop)).toBe(0);
	await expect(trail).toHaveCount(0);
	await page.goForward();
	await expect(page.locator('[data-save-detail-overlay]')).toHaveCount(2);
	await expect(trail.getByRole('button')).toHaveCount(1);
});

test('crowded image trail stays between the controls and opens around the pointer', async ({
	page
}) => {
	await page.setViewportSize({ width: 800, height: 1000 });
	await mockApi(page, { related: true });
	await page.goto('/explore/general');
	await page.locator('a.block:has(img[src*="bafy1"])').click();
	const active = page.locator('[data-save-detail-overlay][data-active="true"]');
	for (let i = 0; i < 9; i++) {
		await active.evaluate((el) => el.scrollTo(0, 1100));
		await active
			.locator('section a.block')
			.nth(i % 3)
			.click();
	}
	await active.evaluate((el) => el.scrollTo(0, 1100));
	const navigation = page.locator('[data-save-detail-navigation]');
	const trail = navigation.getByRole('navigation', { name: 'Image trail' });
	await expect(trail.getByRole('button')).toHaveCount(10);
	await trail.getByRole('button').last().hover();
	const actions = active.locator('[data-desktop-actions]');
	await expect
		.poll(async () => {
			const box = (await trail.boundingBox())!;
			return box.x + box.width <= (await actions.boundingBox())!.x - 11;
		})
		.toBe(true);
	await expect(trail).toHaveCSS('overflow-x', 'clip');
	await expect(trail).toHaveCSS('overflow-y', 'clip');
	await expect
		.poll(async () => {
			const box = (await trail.boundingBox())!;
			return Math.abs(box.width - ((await actions.boundingBox())!.x - box.x - 12));
		})
		.toBeLessThan(1);
	const box = (await trail.boundingBox())!;
	for (const fraction of [0.15, 0.5, 0.85]) {
		await page.mouse.move(box.x + box.width * fraction, box.y + box.height / 2);
		await expect
			.poll(() =>
				trail
					.locator('button:hover')
					.evaluateAll((buttons) =>
						buttons.some((button) => new DOMMatrix(getComputedStyle(button).transform).a > 1.15)
					)
			)
			.toBe(true);
		await expect
			.poll(() =>
				trail.getByRole('button').evaluateAll((buttons) => {
					const scales = buttons.map(
						(button) => new DOMMatrix(getComputedStyle(button).transform).a
					);
					return Math.max(...scales);
				})
			)
			.toBeGreaterThan(1.15);
		await expect
			.poll(() =>
				trail.getByRole('button').evaluateAll((buttons) => {
					const parent = buttons[0].closest('nav')!.getBoundingClientRect();
					return buttons.every((button) => {
						const rect = button.getBoundingClientRect();
						return rect.left >= parent.left && rect.right <= parent.right;
					});
				})
			)
			.toBe(true);
	}
	// Small cursor movements within one image must produce distinct zoom levels.
	const middleX = box.x + box.width * 0.5;
	await page.mouse.move(middleX, box.y + box.height / 2);
	await expect
		.poll(() =>
			trail.getByRole('button').evaluateAll((buttons) =>
				buttons.every((button) => {
					const transform = new DOMMatrix(getComputedStyle(button).transform);
					return (
						Math.abs(transform.e - parseFloat(button.style.getPropertyValue('--x'))) < 0.1 &&
						Math.abs(transform.a - Number(button.style.getPropertyValue('--scale'))) < 0.005
					);
				})
			)
		)
		.toBe(true);
	const hovered = trail.locator('button:hover');
	await expect(hovered).toHaveCount(1);
	const label = await hovered.getAttribute('aria-label');
	const target = trail.getByRole('button', { name: label!, exact: true });
	await expect(target).toHaveCSS('cursor', 'pointer');
	await expect(target).toHaveCSS('border-top-width', '0px');
	await expect
		.poll(() => target.evaluate((node) => getComputedStyle(node, '::after').borderTopWidth))
		.toBe('1px');
	await expect
		.poll(() =>
			target.evaluate(
				(node) =>
					getComputedStyle(node, '::after').borderTopColor ===
					getComputedStyle(node).getPropertyValue('--background').trim()
			)
		)
		.toBe(true);
	const scale = await target.evaluate((node) =>
		Number((node as HTMLElement).style.getPropertyValue('--scale'))
	);
	await page.mouse.move(middleX + 3, box.y + box.height / 2);
	await expect
		.poll(() =>
			target.evaluate((node) => Number((node as HTMLElement).style.getPropertyValue('--scale')))
		)
		.not.toBe(scale);
	await expect(trail.getByRole('button').first()).toHaveCSS('border-top-width', '0px');
	await expect
		.poll(() =>
			trail
				.getByRole('button')
				.first()
				.evaluate((node) => getComputedStyle(node, '::after').borderTopWidth)
		)
		.toBe('1px');
	const sweep = await trail.evaluate(async (nav) => {
		const rect = nav.getBoundingClientRect();
		const buttons = [...nav.querySelectorAll('button')];
		const samples: { x: number; targets: number[]; scales: number[]; top: number[] }[] = [];
		for (const start of [0, Math.max(0, rect.width - 64)]) {
			for (let offset = 0; offset <= 64; offset++) {
				const x = start + offset;
				nav.dispatchEvent(
					new PointerEvent('pointermove', {
						pointerType: 'mouse',
						clientX: rect.left + x,
						bubbles: true
					})
				);
				await new Promise(requestAnimationFrame);
				samples.push({
					x,
					targets: buttons.map((b) => parseFloat(b.style.getPropertyValue('--x'))),
					scales: buttons.map((b) => Number(b.style.getPropertyValue('--scale'))),
					top: buttons.map((b) => b.getBoundingClientRect().top)
				});
			}
		}
		return samples;
	});
	for (const [i, sample] of sweep.entries()) {
		expect(sample.targets.every(Number.isFinite)).toBe(true);
		expect(Math.max(...sample.top) - Math.min(...sample.top)).toBeLessThan(0.5);
		if (i && sample.x - sweep[i - 1].x === 1) {
			for (let j = 0; j < sample.targets.length; j++) {
				expect(Math.abs(sample.targets[j] - sweep[i - 1].targets[j])).toBeLessThan(4);
				expect(Math.abs(sample.scales[j] - sweep[i - 1].scales[j])).toBeLessThan(0.05);
			}
		}
	}
	expect(Math.max(...sweep.flatMap((s) => s.scales))).toBeCloseTo(1.22, 2);

	await trail.getByRole('button').nth(4).focus();
	await page.mouse.move(400, 500);
	await page.keyboard.press('Enter');
	await expect(page.locator('[data-save-detail-overlay]')).toHaveCount(5);
});

test('deep related navigation bounds rendered details', async ({ page }) => {
	test.setTimeout(180_000);
	await page.setViewportSize({ width: 1200, height: 900 });
	await mockApi(page, { related: true });
	const cdp = await page.context().newCDPSession(page);
	await cdp.send('Performance.enable');
	await page.goto('/explore/general');
	await page.locator('a.block:has(img[src*="bafy1"])').click();
	const active = page.locator('[data-save-detail-overlay][data-active="true"]');
	const samples = [];
	for (let depth = 1; depth <= 40; depth++) {
		await active.evaluate((el) => el.scrollTo(0, 1100));
		await expect(page.locator('[data-save-detail-overlay][data-rendered="true"]')).toHaveCount(
			Math.min(depth, 2)
		);
		if ([1, 10, 20, 40].includes(depth)) {
			await cdp.send('HeapProfiler.collectGarbage');
			const { metrics } = await cdp.send('Performance.getMetrics');
			const dom = await cdp.send('Memory.getDOMCounters');
			samples.push({
				depth,
				...dom,
				...Object.fromEntries(
					metrics
						.filter((m) =>
							['JSHeapUsedSize', 'LayoutDuration', 'RecalcStyleDuration', 'TaskDuration'].includes(
								m.name
							)
						)
						.map((m) => [m.name, m.value])
				)
			});
			console.log('DEEP_PROFILE', JSON.stringify(samples.at(-1)));
		}
		if (depth < 40)
			await active
				.locator('section a.block')
				.nth(depth % 10)
				.click();
	}
	await test.info().attach('deep-profile', {
		body: JSON.stringify(samples, null, 2),
		contentType: 'application/json'
	});
});

test('trail expands only near thumbnails and shows a delayed optimized preview', async ({
	page
}) => {
	await page.setViewportSize({ width: 1440, height: 1000 });
	await mockApi(page, { related: true });
	const cdnFeed = feed.map((save) => ({
		...save,
		content: {
			...save.content,
			imageUrl: `https://cdn.currents.is/img/${me.did}/${save.content.blobCid}`
		}
	}));
	await page.route('**/xrpc/is.currents.feed.getFeed*', (route) =>
		route.fulfill({ json: { feed: cdnFeed, cursor: null } })
	);
	await page.route('https://cdn.currents.is/img/**', (route) =>
		route.fulfill({
			contentType: 'image/svg+xml',
			body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"><rect width="400" height="500" fill="steelblue"/></svg>'
		})
	);
	await page.goto('/explore/general');
	await page.locator('a.block:has(img[src*="bafy1"])').click();
	await page
		.locator('[data-save-detail-overlay][data-active="true"]')
		.evaluate((el) => el.scrollTo(0, 1100));
	const trail = page.getByRole('navigation', { name: 'Image trail' });
	await expect(trail).toBeVisible();
	const bounds = (await trail.boundingBox())!;
	await page.mouse.move(bounds.x + 38, bounds.y + 145);
	await expect(trail).not.toHaveClass(/open/);
	const thumbnail = trail.getByRole('button').first();
	await expect(thumbnail.locator('img')).toHaveAttribute('sizes', '48px');
	await expect(thumbnail.locator('img')).toHaveAttribute('src', /optimizer=image/);
	await expect(thumbnail.locator('img')).toHaveAttribute('srcset', /width=320/);
	await thumbnail.hover();
	await expect(trail).toHaveClass(/open/);
	const preview = page.locator('[data-trail-preview]');
	await page.waitForTimeout(200);
	await expect(preview).toHaveCount(0);
	await expect(preview).toBeVisible();
	await expect(preview.locator('img')).toHaveAttribute('src', /optimizer=image/);
	await expect(preview.locator('img')).toHaveAttribute('sizes', '280px');
	const imageBox = (await preview.locator('img').boundingBox())!;
	expect(imageBox.width).toBeGreaterThan(150);
	expect(imageBox.y).toBeGreaterThan(
		(await thumbnail.boundingBox())!.y + (await thumbnail.boundingBox())!.height
	);
	const thumbnailBox = (await thumbnail.boundingBox())!;
	// A small buffer around the thumbnail strip keeps it open.
	await page.mouse.move(thumbnailBox.x + thumbnailBox.width / 2, bounds.y + 120);
	await expect(trail).toHaveClass(/open/);
	// Entering the preview is outside that strip and must close both layers.
	await page.mouse.move(imageBox.x + imageBox.width / 2, imageBox.y + imageBox.height / 2);
	await expect(preview).toHaveCount(0);
	await expect(trail).not.toHaveClass(/open/);
	await page.waitForTimeout(600);
	await expect(preview).toHaveCount(0);
	await expect(page.locator('[data-slot="hover-card-content"]')).toHaveCount(0);
	await thumbnail.hover();
	await expect(preview).toBeVisible();
	await expect(preview.locator('img')).toBeVisible();
	await page.mouse.move(1200, 800);
	await expect(page.locator('[data-slot="hover-card-content"]')).toHaveCount(0);
});
