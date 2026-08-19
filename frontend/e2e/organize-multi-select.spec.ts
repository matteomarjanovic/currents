import { test, expect, type Page } from '@playwright/test';

// Multi-select / bulk actions in organize mode: enter via the header "Select"
// toggle or a tile's context menu, then act on the whole selection (copy / move /
// download / attribution / labels).

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };

const col = (rkey: string) => `at://did:plc:test/is.currents.feed.collection/${rkey}`;
const INTERIORS = col('interiors');

const COLLECTIONS = [
	{ uri: INTERIORS, name: 'Interiors', saveCount: 3, createdAt: '2026-01-01T00:00:00Z' }
];

// Three image saves for the library grid.
function librarySaves(withMembership = false) {
	return [1, 2, 3].map((n) => ({
		uri: `at://did:plc:test/is.currents.feed.save/save${n}`,
		author: me,
		createdAt: `2026-02-0${n}T00:00:00Z`,
		content: {
			$type: 'is.currents.content.defs#imageView',
			imageUrl: `https://example.com/img${n}.jpg`,
			blobCid: `cid${n}`,
			width: 400,
			height: 500
		},
		...(withMembership
			? {
					viewer: {
						saves: [
							{ collectionUri: '', saveUri: `at://did:plc:test/is.currents.feed.save/save${n}` },
							...(n === 1
								? [
										{
											collectionUri: INTERIORS,
											saveUri: 'at://did:plc:test/is.currents.feed.save/interior1'
										}
									]
								: [])
						]
					}
				}
			: {})
	}));
}

type Calls = { labelBulk: string[]; resave: string[]; deleted: string[] };

async function mockApi(page: Page, calls: Calls, withMembership = false) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const req = route.request();
		const url = req.url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/save/labels/bulk') && req.method() === 'PUT') {
			calls.labelBulk.push(req.postData() ?? '');
			return json({ applied: 2, skipped: 0, failed: 0 });
		}
		if (url.endsWith('/resave') && req.method() === 'POST') {
			calls.resave.push(req.postData() ?? '');
			return json({ uri: 'at://did:plc:test/is.currents.feed.save/new', cid: 'newcid' });
		}
		if (url.includes('/api/save/') && req.method() === 'DELETE') {
			calls.deleted.push(url.split('/').pop() ?? '');
			return json({});
		}
		if (url.includes('/api/me/role')) return json({ role: 'user' });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('/api/supporter/status'))
			return json({ active: true, subscribed: true, colorTrialsLeft: 0 });
		if (url.includes('getActorCollections')) return json({ collections: COLLECTIONS });
		if (url.includes('getFavouriteCollections')) return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		if (url.includes('getLibrarySaves'))
			return json({ saves: librarySaves(withMembership), cursor: null });
		return json({ saves: [], cursor: null });
	});
}

test('header toggle enters select mode and bulk-labels the selection', async ({ page }) => {
	const calls: Calls = { labelBulk: [], resave: [], deleted: [] };
	await mockApi(page, calls);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	await expect(page.locator('[data-uri]').first()).toBeVisible();
	await page.getByRole('button', { name: 'Select' }).click();

	// Select the first two tiles.
	await page.locator('[data-uri]').nth(0).click();
	await page.locator('[data-uri]').nth(1).click();
	await expect(page.getByText('2 selected')).toBeVisible();

	// Apply a self-label to both.
	await page.getByRole('button', { name: 'Labels' }).click();
	await page.getByRole('button', { name: 'Porn', exact: true }).click();
	await page.getByRole('button', { name: 'Apply to 2' }).click();

	await expect.poll(() => calls.labelBulk.length).toBeGreaterThan(0);
	const body = JSON.parse(calls.labelBulk[0]);
	expect(body.labels).toContain('porn');
	expect(body.rkeys).toEqual(['save1', 'save2']);
	// The bar exits after a terminal action.
	await expect(page.getByText('2 selected')).toBeHidden();
});

test('the tile menu Select enters the mode with that tile picked, and Copy resaves', async ({
	page
}) => {
	const calls: Calls = { labelBulk: [], resave: [], deleted: [] };
	await mockApi(page, calls);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	const firstTile = page.locator('[data-uri]').first();
	await expect(firstTile).toBeVisible();
	await firstTile.click({ button: 'right' });
	await page.getByRole('menuitem', { name: 'Select' }).click();
	await expect(page.getByText('1 selected')).toBeVisible();

	// Copy to a collection: resave the one selected save into it.
	await page.getByRole('button', { name: 'Copy' }).click();
	await page.getByRole('button', { name: 'Interiors' }).click();

	await expect.poll(() => calls.resave.length).toBeGreaterThan(0);
	const body = JSON.parse(calls.resave[0]);
	expect(body.collectionUri).toBe(INTERIORS);
});

test('My library can remove one membership without losing the deduplicated image', async ({
	page
}) => {
	const calls: Calls = { labelBulk: [], resave: [], deleted: [] };
	await mockApi(page, calls, true);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	const firstTile = page.locator('[data-uri]').first();
	await expect(firstTile).toBeVisible();
	await firstTile.click({ button: 'right' });
	await page.getByRole('menuitem', { name: 'Remove from…' }).hover();
	await page.getByRole('menuitem', { name: 'Interiors' }).click();

	await expect.poll(() => calls.deleted).toContain('interior1');
	await expect(firstTile).toBeVisible();
});

// Mobile has no room for the bar, so it gets a floating pill and one drawer whose
// view swaps in place. The swap is the point: stacking a second drawer over the
// menu is what leaks the body scroll-lock (see scroll-lock.spec.ts).
test('mobile: the floating pill opens the menu, and Copy swaps the drawer in place', async ({
	page
}) => {
	const calls: Calls = { labelBulk: [], resave: [], deleted: [] };
	await mockApi(page, calls);
	await page.goto('/organize');

	await expect(page.locator('[data-uri]').first()).toBeVisible();
	await page.getByRole('button', { name: 'Select' }).click();
	await page.locator('[data-uri]').nth(0).click();
	await page.locator('[data-uri]').nth(1).click();

	// The bar belongs to desktop only; mobile carries the count on the pill.
	await expect(page.getByText('2 selected')).toBeHidden();
	const pill = page.getByRole('button', { name: 'Bulk actions (2)' });
	await expect(pill).toBeVisible();
	await pill.click();

	await expect(page.getByRole('button', { name: 'Copy to collection' })).toBeVisible();
	await page.getByRole('button', { name: 'Copy to collection' }).click();

	// Same drawer, new view — the menu is gone and the destination list is in its place.
	await expect(page.getByRole('button', { name: 'Copy to collection' })).toBeHidden();
	await page.getByRole('button', { name: 'Interiors' }).click();

	await expect.poll(() => calls.resave.length).toBeGreaterThan(0);
	expect(JSON.parse(calls.resave[0]).collectionUri).toBe(INTERIORS);

	// The drawer closed behind the action and left the page scrollable.
	await expect(page.locator('[data-vaul-drawer]')).toHaveCount(0);
	await expect
		.poll(() => page.evaluate(() => getComputedStyle(document.body).overflow), { timeout: 4000 })
		.not.toBe('hidden');
});

// Regression: when the drawer closes itself (tap outside, swipe down) vaul writes
// its own `open` state. With a one-way `open` prop that write never reached us —
// mobileView stayed on 'menu', so re-tapping the pill assigned the same value and
// reopened nothing. Only a two-way binding observes it.
test('mobile: dismissing the drawer lets the pill reopen it', async ({ page }) => {
	const calls: Calls = { labelBulk: [], resave: [], deleted: [] };
	await mockApi(page, calls);
	await page.goto('/organize');

	await expect(page.locator('[data-uri]').first()).toBeVisible();
	await page.getByRole('button', { name: 'Select' }).click();
	await page.locator('[data-uri]').nth(0).click();

	const pill = page.getByRole('button', { name: /Bulk actions/ });
	const menuItem = page.getByRole('button', { name: 'Copy to collection' });

	await pill.click();
	await expect(menuItem).toBeVisible();

	// Tap the overlay above the sheet to dismiss it.
	await page.mouse.click(20, 30);
	await expect(menuItem).toBeHidden();

	// The pill must reopen it.
	await pill.click();
	await expect(menuItem).toBeVisible();
});

// The bar is a sibling of the rounded panel, not a strip inside it: the panel is
// flex-1, so it gives up the height the bar takes. Rendering it inside the panel
// again (or wrapping the inset, which breaks its peer-* gutter) would regress this.
test('desktop: the action bar sits outside the panel and shrinks it', async ({ page }) => {
	const calls: Calls = { labelBulk: [], resave: [], deleted: [] };
	await mockApi(page, calls);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	const panel = page.locator('main[data-slot="sidebar-inset"] > div').first();
	await expect(page.locator('[data-uri]').first()).toBeVisible();
	const before = (await panel.boundingBox())!.height;

	await page.getByRole('button', { name: 'Select' }).click();
	await page.locator('[data-uri]').nth(0).click();
	await expect(page.getByText('1 selected')).toBeVisible();

	// The bar is its own child of the inset, not a descendant of the panel.
	await expect(page.locator('main[data-slot="sidebar-inset"] > div')).toHaveCount(2);
	expect(await panel.evaluate((el) => !!el.textContent?.includes('Select all loaded'))).toBe(false);

	// And the panel gave up the height the bar took.
	await expect.poll(async () => (await panel.boundingBox())!.height).toBeLessThan(before);
});

// The slide must be `|global`: transitions are local by default, and the block this
// element lives in is not the one that changes (the page's {#if selectMode} is), so
// a local transition silently never plays.
test('desktop: the action bar animates in', async ({ page }) => {
	const calls: Calls = { labelBulk: [], resave: [], deleted: [] };
	await mockApi(page, calls);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	// Svelte 5 drives `css` transitions through the Web Animations API, so record
	// the calls rather than racing the 200ms window with a computed-style read.
	await page.evaluate(() => {
		const w = window as unknown as { __anims: string[] };
		w.__anims = [];
		const orig = Element.prototype.animate;
		Element.prototype.animate = function (this: Element, ...args: unknown[]) {
			w.__anims.push((this as HTMLElement).className ?? '');
			// eslint-disable-next-line @typescript-eslint/no-explicit-any
			return orig.apply(this, args as any);
		};
	});

	await expect(page.locator('[data-uri]').first()).toBeVisible();
	await page.getByRole('button', { name: 'Select' }).click();

	// The bar's own element animated in.
	await expect
		.poll(() =>
			page.evaluate(() =>
				(window as unknown as { __anims: string[] }).__anims.filter((c) =>
					c.includes('backdrop-blur-sm')
				)
			)
		)
		.not.toHaveLength(0);
});
