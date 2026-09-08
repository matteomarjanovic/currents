import { test, expect, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const collectionUri = (rkey: string) => `at://did:plc:test/is.currents.feed.collection/${rkey}`;
const saveUri = (rkey: string) => `at://did:plc:test/is.currents.feed.save/${rkey}`;
const INTERIORS = collectionUri('interiors');
const TRAVEL = collectionUri('travel');
const COLLECTIONS = [
	{ uri: INTERIORS, name: 'Interiors', saveCount: 1, createdAt: '2026-01-01T00:00:00Z' },
	{ uri: TRAVEL, name: 'Travel', saveCount: 1, createdAt: '2026-01-02T00:00:00Z' }
];

type Calls = { resave: string[]; deleted: string[] };

function result(memberships: { collectionUri: string; saveUri: string }[]) {
	return {
		uri: memberships[0].saveUri,
		author: me,
		createdAt: '2026-02-01T00:00:00Z',
		content: {
			$type: 'is.currents.content.defs#imageView',
			imageUrl: 'https://example.com/result.jpg',
			blobCid: 'cid-result',
			width: 400,
			height: 500
		},
		viewer: { saves: memberships }
	};
}

async function mockApi(
	page: Page,
	calls: Calls,
	memberships: { collectionUri: string; saveUri: string }[]
) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const request = route.request();
		const url = request.url();
		const json = (body: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
		if (url.endsWith('/resave') && request.method() === 'POST') {
			calls.resave.push(request.postData() ?? '');
			return json({ uri: saveUri('moved') });
		}
		if (url.includes('/api/save/') && request.method() === 'DELETE') {
			calls.deleted.push(url.split('/').pop() ?? '');
			return route.fulfill({ status: 204 });
		}
		if (url.includes('/api/me/role')) return json({ role: 'user' });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('/api/supporter/status'))
			return json({ active: true, subscribed: true, colorTrialsLeft: 0 });
		if (url.includes('getActorCollections')) return json({ collections: COLLECTIONS });
		if (url.includes('getFavouriteCollections')) return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		if (url.includes('/api/preferences')) return json({ lastSaveRemovalAction: 'ask' });
		if (url.includes('searchLibrarySaves')) return json({ saves: [result(memberships)] });
		return json({ saves: [] });
	});
}

async function search(page: Page, collections: string[]) {
	await expect(page.getByRole('button', { name: 'Select' })).toBeVisible();
	await page.keyboard.press('Control+k');
	const dialog = page.getByRole('dialog', { name: 'Search your library' });
	await dialog.getByPlaceholder('Search your images…').fill('chair');
	for (const uri of collections) {
		const name = COLLECTIONS.find((collection) => collection.uri === uri)!.name;
		await dialog.getByText(name, { exact: true }).click();
	}
	await dialog.getByText('Search for “chair”').click();
	await expect(page.locator('[data-uri]').first()).toBeVisible();
}

test('search across collections asks which matching membership to remove', async ({ page }) => {
	const calls: Calls = { resave: [], deleted: [] };
	await mockApi(page, calls, [
		{ collectionUri: INTERIORS, saveUri: saveUri('interiors') },
		{ collectionUri: TRAVEL, saveUri: saveUri('travel') }
	]);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');
	await search(page, [INTERIORS, TRAVEL]);

	await page.getByRole('button', { name: 'Select' }).click();
	await page.locator('[data-uri]').first().click();
	await expect(page.getByRole('button', { name: 'Move', exact: true })).toBeHidden();
	await expect(page.getByRole('button', { name: 'Remove from collection' })).toBeHidden();
	await page.getByRole('button', { name: 'Done' }).click();

	await page.locator('[data-uri]').first().click({ button: 'right' });
	await expect(page.getByRole('menuitem', { name: 'Move to collection' })).toBeHidden();
	await page.getByRole('menuitem', { name: 'Remove from…' }).hover();
	await page.getByRole('menuitem', { name: 'Interiors' }).click();

	await expect.poll(() => calls.deleted).toContain('interiors');
});

test('search in one owned collection offers direct remove and move', async ({ page }) => {
	const calls: Calls = { resave: [], deleted: [] };
	await mockApi(page, calls, [{ collectionUri: INTERIORS, saveUri: saveUri('interiors') }]);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');
	await search(page, [INTERIORS]);

	await page.getByRole('button', { name: 'Select' }).click();
	await page.locator('[data-uri]').first().click();
	await expect(page.getByRole('button', { name: 'Move', exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Remove from collection' })).toBeVisible();
	await page.getByRole('button', { name: 'Done' }).click();

	await page.locator('[data-uri]').first().click({ button: 'right' });
	await expect(page.getByRole('menuitem', { name: 'Remove from Interiors' })).toBeVisible();
	await page.getByRole('menuitem', { name: 'Move to collection' }).hover();
	await page.getByRole('button', { name: 'Travel Public', exact: true }).click();

	await expect.poll(() => calls.resave.length).toBe(1);
	expect(JSON.parse(calls.resave[0])).toEqual({
		saveUri: saveUri('interiors'),
		collectionUri: TRAVEL
	});
	await expect.poll(() => calls.deleted).toContain('interiors');
});
