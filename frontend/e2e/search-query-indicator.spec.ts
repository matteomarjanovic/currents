import { test, expect, type Page } from '@playwright/test';

// While a search is open the top bar keeps showing what's being searched: the
// query beside the lens on desktop, a chip above the bottom cluster on mobile
// (no room beside the lens there), and a lens painted with the searched color.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };

async function mockApi(page: Page) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('/api/supporter/status'))
			return json({ active: true, subscribed: true, colorTrialsLeft: 0 });
		if (url.includes('searchSaves')) return json({ saves: [], cursor: null });
		if (url.includes('features/seen')) return json({ seen: ['color-search'] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

const lens = (page: Page) => page.locator('button[aria-label="Search"]:visible');

test('a hybrid search shows its text, hex and colored lens on mobile', async ({ page }) => {
	await mockApi(page);
	await page.goto('/search/color/e63946?q=sunset');

	await expect(page.getByRole('button', { name: 'sunset #E63946' })).toBeVisible();
	await expect(lens(page).locator('circle[fill="#e63946"]')).toHaveCount(1);
});

test('the mobile chip is absent without an active search', async ({ page }) => {
	await mockApi(page);
	await page.goto('/explore');

	await expect(lens(page)).toBeVisible();
	await expect(lens(page).locator('circle[fill]')).toHaveCount(0);
	await expect(page.getByRole('button', { name: /#/ })).toHaveCount(0);
});

// Text search is public, so the chip has to work without a session too — it
// rides above the logged-out bottom cluster (log in / theme / search).
test('a logged-out viewer reaches a text search and sees the chip', async ({ page }) => {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		if (url.includes('/api/me') || url.includes('/api/supporter/status'))
			return route.fulfill({ status: 401, body: '' });
		return route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ saves: [], cursor: null })
		});
	});
	await page.goto('/search/saves/mountain%20cabin');

	await expect(page.getByRole('button', { name: 'mountain cabin' })).toBeVisible();
	await expect(page).toHaveURL(/\/search\/saves\/mountain/);
});

test('the desktop search button carries the query beside the lens', async ({ page }) => {
	await mockApi(page);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/search/saves/mountain%20cabin');

	await expect(lens(page)).toContainText('mountain cabin');
});
