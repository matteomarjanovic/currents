import { test, expect, type Page } from '@playwright/test';

// /support-us is public, so a logged-out visitor can click a tier. The page
// already routes that to promptLogin(); this pins that the prompt is actually
// mounted on this route — it lives outside the (with-navbar)/(without-navbar)
// layouts that mount it for the rest of the app.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };

async function mockApi(page: Page, loggedIn: boolean) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/api/me'))
			return loggedIn ? json(me) : route.fulfill({ status: 401, body: '' });
		if (url.includes('/api/supporter/status'))
			return loggedIn
				? json({ active: false, subscribed: false, colorTrialsLeft: 0 })
				: route.fulfill({ status: 401, body: '' });
		if (url.includes('/api/supporter/stats'))
			return json({ totalUsers: 120, supporters: 4, byProduct: {} });
		return json({});
	});
}

// Both tiers run through the same subscribe(), so the monthly one covers it.
const monthlyTier = (page: Page) => page.getByRole('button', { name: /\$7\b/ });

// The page is prerendered with the tier buttons already enabled, so a click can
// land before hydration and do nothing. The transparency figures start as '—'
// and only resolve once the client has mounted and fetched — wait on those.
async function load(page: Page, loggedIn: boolean) {
	await mockApi(page, loggedIn);
	await page.goto('/support-us');
	await expect(page.getByText('People on Currents').locator('..')).toContainText('120');
}

test('a logged-out visitor clicking a tier gets the login prompt', async ({ page }) => {
	await load(page, false);

	await expect(monthlyTier(page)).toBeEnabled();
	await monthlyTier(page).click();

	await expect(page.getByText('Log in to continue')).toBeVisible();
});

test('a logged-in visitor clicking a tier goes straight to checkout', async ({ page }) => {
	await load(page, true);

	await monthlyTier(page).click();
	await page.waitForTimeout(800);

	await expect(page.getByText('Log in to continue')).toHaveCount(0);
});
