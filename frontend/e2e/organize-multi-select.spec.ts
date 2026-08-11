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
function librarySaves() {
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
		}
	}));
}

type Calls = { labelBulk: string[]; resave: string[] };

async function mockApi(page: Page, calls: Calls) {
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
		if (url.includes('/api/me/role')) return json({ role: 'user' });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('/api/supporter/status'))
			return json({ active: true, subscribed: true, colorTrialsLeft: 0 });
		if (url.includes('getActorCollections')) return json({ collections: COLLECTIONS });
		if (url.includes('getFavouriteCollections')) return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		if (url.includes('getLibrarySaves')) return json({ saves: librarySaves(), cursor: null });
		return json({ saves: [], cursor: null });
	});
}

test('header toggle enters select mode and bulk-labels the selection', async ({ page }) => {
	const calls: Calls = { labelBulk: [], resave: [] };
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
	const calls: Calls = { labelBulk: [], resave: [] };
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
