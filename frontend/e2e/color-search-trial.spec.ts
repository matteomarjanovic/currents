import { test, expect, type Page } from '@playwright/test';

// Color search is the one supporter feature non-supporters can sample: the
// client lets them through while the server still reports trial colors left,
// and raises the paywall once they're spent. The count itself is owned and
// enforced server-side (requireColorSearch in appview/supporter.go); here it's
// mocked to pin the two client branches.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };

// `marked` collects the feature keys the client dismisses, so a test can assert
// the announcement was cleared server-side and not just visually.
async function mockApi(page: Page, colorTrialsLeft: number, marked: string[] = []) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('/api/supporter/status'))
			return json({ active: false, subscribed: false, colorTrialsLeft });
		if (url.includes('searchSavesByColor')) return json({ saves: [], cursor: null });
		if (route.request().method() === 'POST' && url.includes('features/seen/')) {
			marked.push(decodeURIComponent(url.split('features/seen/')[1]));
			return route.fulfill({ status: 204, body: '' });
		}
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

// /explore rather than / because the landing page's search button opens an
// inline search bar instead of the command dialog.
async function openColorPanel(page: Page) {
	await page.goto('/explore');
	await page.locator('button[aria-label="Search"]:visible').first().click();
	await page.getByRole('button', { name: 'Search by color' }).click();
}

test('a non-supporter with trial colors left runs a color search', async ({ page }) => {
	await mockApi(page, 2);
	await openColorPanel(page);

	await expect(page.getByText('2 free color searches left')).toBeVisible();

	await page.getByRole('button', { name: 'Pick #e63946' }).click();
	await page.getByRole('button', { name: 'Search this color' }).click();

	await expect(page).toHaveURL(/\/search\/color\/e63946$/);
	await expect(page.getByText('Support Currents')).toHaveCount(0);
});

test('Enter starts the color search, like a text search', async ({ page }) => {
	await mockApi(page, 2);
	await openColorPanel(page);

	await page.getByRole('button', { name: 'Pick #457b9d' }).click();
	// Enter from the text field runs it — no need to reach the "Search this color" button.
	await page.getByPlaceholder('Describe it too (optional)…').press('Enter');

	await expect(page).toHaveURL(/\/search\/color\/457b9d$/);
});

// The "new feature" trail: a dot on the search button leads to a dot on the
// palette toggle, and opening the color panel clears the flag for good.
test('the color-search announcement clears when the color panel opens', async ({ page }) => {
	const marked: string[] = [];
	await mockApi(page, 5, marked);
	await page.goto('/explore');

	const searchButton = page.locator('button[aria-label="Search"]:visible').first();
	await expect(searchButton.locator('span[aria-label="New feature available"]')).toBeVisible();

	await searchButton.click();
	const colorToggle = page.getByRole('button', { name: 'Search by color' });
	await expect(colorToggle.locator('span[aria-label="New feature available"]')).toBeVisible();

	await colorToggle.click();
	await expect(colorToggle.locator('span[aria-label="New feature available"]')).toHaveCount(0);
	await expect.poll(() => marked).toContain('color-search');
});

// A logged-out viewer has no account to attach a subscription to, so both the
// client gate and the endpoint's 403 lead to the login prompt, never the paywall.
async function mockLoggedOut(page: Page) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		if (url.includes('/api/me') || url.includes('/api/supporter/status'))
			return route.fulfill({ status: 401, body: '' });
		if (url.includes('searchSavesByColor')) return route.fulfill({ status: 403, body: '' });
		return route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({})
		});
	});
}

test('a logged-out viewer gets the login prompt instead of the paywall', async ({ page }) => {
	await mockLoggedOut(page);
	await openColorPanel(page);

	await page.getByRole('button', { name: 'Search this color' }).click();

	await expect(page.getByText('Log in to continue')).toBeVisible();
	await expect(page.getByText('Support Currents')).toHaveCount(0);
	await expect(page).not.toHaveURL(/\/search\/color\//);
});

test('a logged-out viewer deep-linking a color search gets the login prompt', async ({ page }) => {
	await mockLoggedOut(page);
	await page.goto('/search/color/e63946');

	await expect(page.getByText('Log in to continue')).toBeVisible();
	await expect(page.getByText('Support Currents')).toHaveCount(0);
});

test('the paywall takes over once the trial colors are spent', async ({ page }) => {
	await mockApi(page, 0);
	await openColorPanel(page);

	await expect(page.getByText('used your free color searches')).toBeVisible();

	await page.getByRole('button', { name: 'Search this color' }).click();

	await expect(page.getByText('Support Currents')).toBeVisible();
	await expect(page).not.toHaveURL(/\/search\/color\//);
});
