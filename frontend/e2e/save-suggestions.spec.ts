import { test, expect, type Page } from '@playwright/test';

test.use({ viewport: { width: 1280, height: 800 }, hasTouch: false, isMobile: false });

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const collection = (rkey: string) => `at://${me.did}/is.currents.feed.collection/${rkey}`;
const FLOWERS = collection('flowers');
const CARS = collection('cars');
const SPORTS_CARS = collection('sports-cars');
const LANDSCAPES = collection('landscapes');

const collections = [
	{ uri: FLOWERS, name: 'Flowers', createdAt: '2026-01-01T00:00:00Z' },
	{ uri: CARS, name: 'Cars', sectionCount: 1, createdAt: '2026-01-02T00:00:00Z' },
	{
		uri: SPORTS_CARS,
		name: 'Sports Cars',
		parentUri: CARS,
		createdAt: '2026-01-03T00:00:00Z'
	},
	{ uri: LANDSCAPES, name: 'Landscapes', createdAt: '2026-01-04T00:00:00Z' }
];

const saves = [1, 2, 3].map((n) => ({
	uri: `at://did:plc:source/is.currents.feed.save/s${n}`,
	author: { did: 'did:plc:source', handle: 'source.bsky.social' },
	createdAt: `2026-02-0${n}T00:00:00Z`,
	content: {
		$type: 'is.currents.content.defs#imageView',
		blobCid: `blob${n}`,
		imageUrl: `${APPVIEW}/img/did:plc:source/blob${n}`,
		width: 400,
		height: 500
	},
	viewer: { saves: [] }
}));

async function mockApi(page: Page, resaves: { saveUri: string; collectionUri: string }[]) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const request = route.request();
		const url = request.url();
		const json = (value: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(value) });
		if (url.includes('/img/')) {
			return route.fulfill({
				status: 200,
				contentType: 'image/svg+xml',
				body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"><rect width="400" height="500" fill="#888"/></svg>'
			});
		}
		if (url.endsWith('/resave') && request.method() === 'POST') {
			resaves.push(request.postDataJSON() as { saveUri: string; collectionUri: string });
			return json({ uri: `at://${me.did}/is.currents.feed.save/new${resaves.length}` });
		}
		if (url.includes('/api/save-suggestions')) {
			const requested = (request.postDataJSON() as { saveUris: string[] }).saveUris;
			const byURI: Record<string, string> = {
				[saves[0].uri]: FLOWERS,
				[saves[1].uri]: LANDSCAPES,
				[saves[2].uri]: LANDSCAPES
			};
			return json({
				suggestions: Object.fromEntries(requested.map((uri) => [uri, byURI[uri]]))
			});
		}
		if (url.includes('/api/feed/preferences')) {
			return json({ excludedCollections: [], defaultFeed: 'general' });
		}
		if (url.includes('/api/preferences')) {
			return json({
				gifAutoplay: true,
				organizeCollectionSort: 'name',
				saveSuggestionMode: 'recommended-then-last-used'
			});
		}
		if (url.includes('/api/me/role')) return json({ role: 'user' });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('getActorCollections')) return json({ collections });
		if (url.includes('getFeed')) return json({ feed: saves, cursor: null });
		if (url.includes('getLibrarySaves')) return json({ saves: [], cursor: null });
		if (url.includes('getFavouriteCollections')) return json({ collections: [] });
		if (url.includes('/api/supporter/status')) {
			return json({ active: true, subscribed: false, colorTrialsLeft: 5 });
		}
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) {
			return json({
				porn: 'blur',
				sexual: 'blur',
				nudity: 'blur',
				graphicMedia: 'blur',
				aiGenerated: 'show'
			});
		}
		return json({});
	});
}

function card(page: Page, n: number) {
	return page
		.locator(`a[href$="/save/s${n}"]`)
		.locator('xpath=ancestor::div[contains(@class,"group")][1]');
}

test('Quick Save recommends, follows successful choices, and resets after leaving Explore', async ({
	page
}) => {
	const resaves: { saveUri: string; collectionUri: string }[] = [];
	await page.addInitScript(
		(lastUsed) => localStorage.setItem('lastUsedCollectionUri', lastUsed),
		SPORTS_CARS
	);
	await mockApi(page, resaves);
	await page.goto('/explore/general');

	const first = card(page, 1);
	await expect(first).toBeVisible();
	await expect(first.getByRole('button', { name: 'Flowers', exact: true })).toBeAttached();
	await first.hover();
	await expect(first.getByRole('button', { name: 'Flowers', exact: true })).toBeVisible();

	// The suggested target and the persistent menu order are independent: the
	// button says Flowers while the previous Sports Cars choice remains first.
	await first.getByRole('button', { name: 'Flowers', exact: true }).click();
	const menuButtons = page.locator('[data-slot="popover-content"]:visible button');
	await expect(menuButtons.nth(2)).toContainText('Sports Cars');
	await page.keyboard.press('Escape');

	await first.getByRole('button', { name: 'Save', exact: true }).click();
	await expect.poll(() => resaves.at(-1)?.collectionUri).toBe(FLOWERS);

	const second = card(page, 2);
	await second.hover();
	// Its own recommendation is Landscapes, but the successful Flowers save now wins.
	await expect(second.getByRole('button', { name: 'Flowers', exact: true })).toBeVisible();
	await second.getByRole('button', { name: 'Flowers', exact: true }).click();
	const secondMenu = page.locator('[data-slot="popover-content"]:visible');
	await secondMenu.getByRole('button', { name: /Cars Public.*section/ }).click();
	await secondMenu.getByRole('button', { name: 'Sports Cars Public', exact: true }).click();
	await expect.poll(() => resaves.at(-1)?.collectionUri).toBe(SPORTS_CARS);

	const third = card(page, 3);
	await third.hover();
	await expect(third.getByRole('button', { name: 'Sports Cars', exact: true })).toBeVisible();

	// Navigate through the real mode switcher so this is a client-side route
	// transition: leaving Explore must clear only the temporary destination.
	await page.locator('button:visible').filter({ hasText: 'Explore' }).first().click();
	await page.getByRole('menuitem').filter({ hasText: 'Organize' }).click();
	await expect(page).toHaveURL(/\/organize/);
	await page.locator('button:visible').filter({ hasText: 'Organize' }).first().click();
	await page.getByRole('menuitem').filter({ hasText: 'Explore' }).click();
	await expect(page).toHaveURL(/\/explore\/general/);

	const returnedThird = card(page, 3);
	await returnedThird.hover();
	await expect(
		returnedThird.getByRole('button', { name: 'Landscapes', exact: true })
	).toBeVisible();
});
