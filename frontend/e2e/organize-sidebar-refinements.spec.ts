import { test, expect, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const collectionUri = (rkey: string) => `at://did:plc:test/is.currents.feed.collection/${rkey}`;
const saveUri = (rkey: string) => `at://did:plc:test/is.currents.feed.save/${rkey}`;
const INTERIORS = collectionUri('interiors');
const COLLECTIONS = [
	{ uri: INTERIORS, name: 'Interiors', saveCount: 1, createdAt: '2026-01-01T00:00:00Z' }
];

function imageSave(uri: string, memberships: { collectionUri: string; saveUri: string }[]) {
	return {
		uri,
		author: me,
		createdAt: '2026-02-01T00:00:00Z',
		content: {
			$type: 'is.currents.content.defs#imageView',
			imageUrl: 'https://example.com/image.jpg',
			blobCid: 'cid-image',
			width: 400,
			height: 500
		},
		viewer: { saves: memberships }
	};
}

type Calls = { resave: string[]; deleted: string[]; unsortedUrls: string[] };

async function mockApi(page: Page, calls: Calls) {
	const collectionSave = saveUri('interiors');
	const unsortedSave = saveUri('unsorted');
	await page.route(`${APPVIEW}/**`, (route) => {
		const request = route.request();
		const url = request.url();
		const json = (body: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
		if (url.endsWith('/resave') && request.method() === 'POST') {
			calls.resave.push(request.postData() ?? '');
			return json({ uri: unsortedSave });
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
		if (url.includes('getUnsortedSaves')) {
			calls.unsortedUrls.push(url);
			return json({
				saves: [imageSave(unsortedSave, [{ collectionUri: '', saveUri: unsortedSave }])]
			});
		}
		if (url.includes('getLibrarySaves')) {
			return json({
				saves: [imageSave(collectionSave, [{ collectionUri: INTERIORS, saveUri: collectionSave }])]
			});
		}
		return json({ saves: [] });
	});
}

test('right sidebar removes a membership and preserves the last image in Unsorted', async ({
	page
}) => {
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	await mockApi(page, calls);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	await page.locator('[data-uri]').first().click();
	await page.getByRole('button', { name: 'Remove from Interiors' }).click();
	await expect(page.getByRole('alertdialog')).toBeVisible();
	await page.getByRole('button', { name: 'Move to Profile' }).click();

	await expect.poll(() => calls.resave.length).toBe(1);
	expect(JSON.parse(calls.resave[0])).toEqual({
		saveUri: saveUri('interiors'),
		collectionUri: ''
	});
	await expect.poll(() => calls.deleted).toContain('interiors');
	await expect(page.getByRole('button', { name: 'Remove from Unsorted' })).toBeVisible();
});

test('Profile (Unsorted) is the first library source and loads only unsorted saves', async ({
	page
}) => {
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	await mockApi(page, calls);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	const profile = page.getByRole('link', { name: 'Profile (Unsorted)' });
	const interiors = page.getByRole('link', { name: 'Interiors' });
	await expect(profile).toBeVisible();
	await expect(interiors).toBeVisible();
	expect((await profile.boundingBox())!.y).toBeLessThan((await interiors.boundingBox())!.y);
	await profile.click();

	await expect(page).toHaveURL(/unsorted=1/);
	await expect(page.getByText('Unsorted', { exact: true }).first()).toBeVisible();
	await expect(page.locator(`[data-uri="${saveUri('unsorted')}"]`)).toBeVisible();
	await expect.poll(() => calls.unsortedUrls.length).toBe(1);
	expect(calls.unsortedUrls[0]).toContain(`actor=${encodeURIComponent(me.did)}`);
});

test('collection pin controls stay below sticky sidebar headers', async ({ page }) => {
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	await mockApi(page, calls);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');

	const pin = page.getByRole('button', { name: 'Pin Interiors' });
	await expect(pin).toBeAttached();
	await expect(pin).toHaveCSS('z-index', 'auto');
});
