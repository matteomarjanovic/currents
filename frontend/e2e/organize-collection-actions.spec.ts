import { test, expect, type Locator, type Page } from '@playwright/test';

// Collection actions in organize mode: right-click on a sidebar row (desktop)
// and long-press on a sidebar row or the three-dots menu beside the breadcrumb (touch).
// "Move into" re-parents a collection — the one operation that writes a new
// `parent` onto the collection record.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };

const uri = (rkey: string) => `at://did:plc:test/is.currents.feed.collection/${rkey}`;
const INTERIORS = uri('interiors');
const TRAVEL = uri('travel');
const KYOTO = uri('kyoto');

// Interiors is a plain root, Travel a root with one section (Kyoto) — enough to
// cover both sides of the single-level rule.
const COLLECTIONS = [
	{ uri: INTERIORS, name: 'Interiors', saveCount: 4, createdAt: '2026-01-01T00:00:00Z' },
	{ uri: TRAVEL, name: 'Travel', saveCount: 9, createdAt: '2026-01-02T00:00:00Z' },
	{ uri: KYOTO, name: 'Kyoto', parentUri: TRAVEL, saveCount: 3, createdAt: '2026-01-03T00:00:00Z' }
];

async function mockApi(page: Page, onPut?: (body: string) => void) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const req = route.request();
		const url = req.url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/collection/') && req.method() === 'PUT') {
			onPut?.(req.postData() ?? '');
			return json({ uri: '', cid: '' });
		}
		if (url.includes('/collection/') && req.method() === 'DELETE') {
			return route.fulfill({ status: 202 });
		}
		if (url.includes('/api/me/role')) return json({ role: 'moderator' });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('/api/supporter/status'))
			return json({ active: true, subscribed: true, colorTrialsLeft: 0 });
		if (url.includes('getActorCollections')) return json({ collections: COLLECTIONS });
		if (url.includes('getFavouriteCollections')) return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		return json({ saves: [], cursor: null });
	});
}

const menu = (page: Page) => page.locator('[data-state=open][data-slot$=menu-content]');

async function longPress(page: Page, row: Locator) {
	const box = (await row.boundingBox())!;
	const touch = await page.context().newCDPSession(page);
	await touch.send('Input.dispatchTouchEvent', {
		type: 'touchStart',
		touchPoints: [{ x: box.x + box.width / 2, y: box.y + box.height / 2 }]
	});
	await expect(menu(page).getByRole('menuitem', { name: 'Pin', exact: true })).toBeVisible();
	await touch.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
	await expect(menu(page)).toBeVisible();
}

for (const name of ['Interiors', 'Kyoto']) {
	test(`mobile: long-pressing ${name} opens actions without dragging or navigating`, async ({
		page
	}) => {
		await mockApi(page);
		await page.goto('/organize');
		await page.getByRole('button', { name: 'Toggle Sidebar' }).tap();
		const sidebar = page.locator('[data-sidebar=sidebar][data-mobile=true]');
		if (name === 'Kyoto') {
			await sidebar.getByRole('button', { name: 'Toggle sections' }).tap();
		}
		const row = sidebar.getByRole('link', { name, exact: true });
		await expect(row).toBeVisible();
		expect(await row.evaluate((element) => (element as HTMLAnchorElement).draggable)).toBe(false);
		await expect(row).toHaveCSS('-webkit-user-drag', 'none');
		await longPress(page, row);
		await expect(page).toHaveURL(/\/organize$/);

		// Tap where Travel sits, even though the menu's dismissal layer covers it.
		const travel = sidebar.getByRole('link', { name: 'Travel', exact: true });
		const travelBox = (await travel.boundingBox())!;
		await page.touchscreen.tap(travelBox.x + 12, travelBox.y + travelBox.height / 2);
		await expect(menu(page)).toHaveCount(0);
		await expect(sidebar).toBeVisible();
		await expect(page).toHaveURL(/\/organize$/);

		// The next tap is an ordinary collection selection.
		await travel.tap();
		await expect(page).toHaveURL(`/organize?c=${encodeURIComponent(TRAVEL)}`);
		await expect(sidebar).toBeHidden();
	});
}

test('mobile: a long-press menu can pin a collection without closing the sidebar', async ({
	page
}) => {
	await mockApi(page);
	await page.goto('/organize');
	await page.getByRole('button', { name: 'Toggle Sidebar' }).tap();
	const sidebar = page.locator('[data-sidebar=sidebar][data-mobile=true]');
	const row = sidebar.getByRole('link', { name: 'Interiors', exact: true });
	await expect(row).toBeVisible();
	await longPress(page, row);

	const pinRequest = page.waitForRequest(
		(request) => request.url().endsWith('/api/collections/pinned') && request.method() === 'PUT'
	);
	await menu(page).getByRole('menuitem', { name: 'Pin', exact: true }).tap();
	expect((await pinRequest).postDataJSON()).toEqual({ collectionUri: INTERIORS, pinned: true });
	await expect(menu(page)).toHaveCount(0);
	await expect(sidebar.getByRole('button', { name: 'Unpin Interiors' })).toBeVisible();
	await expect(page).toHaveURL(/\/organize$/);
});

test('desktop: right-clicking a collection row opens its actions', async ({ page }) => {
	await mockApi(page);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	await page.getByRole('link', { name: 'Interiors' }).click({ button: 'right' });

	await expect(menu(page).getByText('Edit')).toBeVisible();
	await expect(menu(page).getByText('Create section')).toBeVisible();
	await expect(menu(page).getByText('Move into')).toBeVisible();
	await expect(menu(page).getByText('Delete')).toBeVisible();
});

test('desktop: a collection holding sections cannot be moved into another', async ({ page }) => {
	await mockApi(page);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	await page.getByRole('link', { name: 'Travel' }).click({ button: 'right' });

	const item = menu(page).getByRole('menuitem').filter({ hasText: 'Move into' });
	await expect(item).toHaveAttribute('data-disabled');
	await expect(item).toContainText("A collection with sections can't become one");
});

test('desktop: moving a collection writes the new parent and nests the row', async ({ page }) => {
	let put = '';
	await mockApi(page, (body) => (put = body));
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	await page.getByRole('link', { name: 'Interiors' }).click({ button: 'right' });
	await menu(page).getByText('Move into').click();
	await page.getByRole('menuitem', { name: 'Travel' }).click();

	await expect.poll(() => put).toContain(`"parent":"${TRAVEL}"`);
	// Nested rows live in the sidebar's sub-menu, revealed by the row's expander.
	await page.getByRole('button', { name: 'Toggle sections' }).click();
	await expect(page.locator('[data-slot=sidebar-menu-sub]').getByText('Interiors')).toBeVisible();
});

test('the delete warning names the sections and saves it takes with it', async ({ page }) => {
	await mockApi(page);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	await page.getByRole('link', { name: 'Travel' }).click({ button: 'right' });
	await menu(page).getByText('Delete').click();

	await expect(page.getByRole('alertdialog')).toContainText(
		'This deletes the collection, its 1 section, and all 9 saves inside them.'
	);
	await page.getByRole('alertdialog').getByRole('button', { name: 'Delete' }).click();
	await expect(page.getByText('Deleting "Travel" and its contents')).toBeVisible();
	await expect(page.getByRole('link', { name: 'Travel', exact: true })).toHaveCount(0);
});

test('mobile: the breadcrumb collapses its ancestors and carries the actions menu', async ({
	page
}) => {
	await mockApi(page);
	await page.goto(`/organize?c=${encodeURIComponent(KYOTO)}`);

	// The trail is "… › Kyoto": both ancestors sit behind the ellipsis so the
	// name and the actions button fit.
	const trail = page.locator('[data-slot=breadcrumb-list]');
	await expect(trail.getByText('Kyoto')).toBeVisible();
	await expect(trail.getByRole('link', { name: 'My library' })).toBeHidden();
	await expect(trail.getByRole('link', { name: 'Travel' })).toBeHidden();
	await expect(page.getByRole('button', { name: 'Collection options' })).toBeVisible();

	await page.getByRole('button', { name: 'Go to a parent collection' }).tap();
	await expect(page.getByRole('menuitem', { name: 'My library' })).toBeVisible();
	await expect(page.getByRole('menuitem', { name: 'Travel' })).toBeVisible();
});

test('desktop: the breadcrumb shows the full trail', async ({ page }) => {
	await mockApi(page);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/organize?c=${encodeURIComponent(KYOTO)}`);

	const trail = page.locator('[data-slot=breadcrumb-list]');
	await expect(trail.getByRole('link', { name: 'My library' })).toBeVisible();
	await expect(trail.getByRole('link', { name: 'Travel' })).toBeVisible();
	await expect(trail.getByText('Kyoto')).toBeVisible();
	await expect(trail.getByRole('button', { name: 'Go to a parent collection' })).toBeHidden();
});

test('mobile: a section can be moved back out to the top level', async ({ page }) => {
	let put = '';
	await mockApi(page, (body) => (put = body));
	await page.goto(`/organize?c=${encodeURIComponent(KYOTO)}`);

	await page.getByRole('button', { name: 'Collection options' }).tap();
	await menu(page).getByText('Move into').tap();
	await page.getByRole('menuitem', { name: 'My library' }).tap();

	await expect.poll(() => put).toContain('"parent":""');
});
