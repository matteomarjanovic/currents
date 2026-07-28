import { test, expect, type Page } from '@playwright/test';

// The button clusters keep one menu open at a time. This only ever broke under
// touch: a tap on a second trigger doesn't reach bits-ui's interact-outside
// handler, so both menus sat open (the second one behind the first). Mouse
// clicks always closed the first, hence the desktop case below as a guard.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };

async function mockApi(page: Page) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/api/me/role')) return json({ role: 'moderator' });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

const openMenus = (page: Page) =>
	page.locator('[data-slot=dropdown-menu-content][data-state=open]');
const trigger = (page: Page, label: string) =>
	page.locator(`button[aria-label="${label}"]:visible`);

// One pair per menu kind: the reported case, its reverse, and one covering the
// mode switcher (whose open state is plumbed through a prop).
const PAIRS: [string, string][] = [
	['Menu', 'Profile menu'],
	['Profile menu', 'Menu'],
	['Add', 'Switch mode']
];

for (const [first, second] of PAIRS) {
	test(`touch: ${first} then ${second} leaves only the second open`, async ({ page }) => {
		await mockApi(page);
		await page.goto('/explore');
		await expect(trigger(page, first)).toBeVisible();

		await trigger(page, first).tap();
		await expect(openMenus(page)).toHaveCount(1);

		await trigger(page, second).tap();
		await expect(openMenus(page)).toHaveCount(1);
	});
}

test('touch: a menu still toggles itself shut', async ({ page }) => {
	await mockApi(page);
	await page.goto('/explore');
	await expect(trigger(page, 'Menu')).toBeVisible();

	await trigger(page, 'Menu').tap();
	await expect(openMenus(page)).toHaveCount(1);

	await trigger(page, 'Menu').tap();
	await expect(openMenus(page)).toHaveCount(0);
});

test('mouse: the desktop cluster opens the menu that was clicked', async ({ page }) => {
	await mockApi(page);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/explore');
	await expect(trigger(page, 'Menu')).toBeVisible();

	await trigger(page, 'Menu').click();
	await trigger(page, 'Profile menu').click();

	await expect(openMenus(page)).toHaveCount(1);
	await expect(openMenus(page)).toContainText('Tester');
});
