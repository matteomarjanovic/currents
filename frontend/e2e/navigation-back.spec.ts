import { test, expect, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const collection = {
	uri: `at://${me.did}/is.currents.feed.collection/architecture`,
	author: me,
	name: 'Architecture',
	description: '',
	previews: [],
	saveCount: 0,
	createdAt: '2026-01-01T00:00:00Z'
};

async function mockApi(page: Page, authenticated = true) {
	await page.route(`${APPVIEW}/**`, async (route) => {
		const url = new URL(route.request().url());
		const json = (value: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(value) });

		if (url.pathname === '/api/me') {
			return authenticated ? json(me) : route.fulfill({ status: 401, body: '' });
		}
		if (url.pathname.endsWith('getProfile')) return json(me);
		if (url.pathname.endsWith('getActorCollections')) {
			return json({
				collections: url.searchParams.get('parent') === collection.uri ? [] : [collection]
			});
		}
		if (url.pathname.endsWith('getCollectionSaves')) {
			return json({ collection, saves: [], cursor: null });
		}
		if (url.pathname.endsWith('getFeed')) return json({ feed: [], cursor: null });
		if (url.pathname === '/api/feed/preferences') {
			return json({ excludedCollections: [], defaultFeed: 'personal' });
		}
		if (url.pathname === '/api/supporter/status') {
			return json({ active: true, subscribed: false, colorTrialsLeft: 5 });
		}
		if (url.pathname.includes('moderation/prefs')) {
			return json({
				porn: 'blur',
				sexual: 'blur',
				nudity: 'blur',
				graphicMedia: 'blur',
				aiGenerated: 'show'
			});
		}
		if (url.pathname.includes('features/seen')) return json({ seen: [] });
		return json({});
	});
}

test('returning from Explore does not loop through the authenticated home redirect', async ({
	page
}) => {
	await mockApi(page);
	await page.goto(`/profile/${me.handle}`);
	await expect(page.getByText('Architecture', { exact: true })).toBeVisible();

	await page.getByRole('link', { name: 'Go to home' }).click();
	await expect(page).toHaveURL(/\/explore\/personal$/);
	await page.goBack();

	await expect(page).toHaveURL(new RegExp(`/profile/${me.handle}$`));
});

test('collection has one back control and it returns to the profile', async ({ page }) => {
	await mockApi(page);
	await page.goto(`/profile/${me.handle}`);
	await page.getByText('Architecture', { exact: true }).click();
	await expect(page).toHaveURL(/\/collection\/architecture$/);

	await expect(page.getByRole('button', { name: 'Go back' })).toHaveCount(1);
	await expect(page.getByRole('button', { name: 'Back', exact: true })).toHaveCount(0);
	await page.getByRole('button', { name: 'Go back' }).click();

	await expect(page).toHaveURL(new RegExp(`/profile/${me.handle}$`));
});

test('collection back can return to the site that opened Currents', async ({ page }) => {
	await mockApi(page, false);
	await page.goto('data:text/html,<title>Previous site</title>Previous site');
	await page.goto(`/profile/${me.handle}/collection/architecture`);
	await expect(page.getByRole('heading', { name: 'Architecture' })).toBeVisible();

	await page.getByRole('button', { name: 'Go back' }).click();

	await expect(page).toHaveURL(/^data:text\/html/);
});
