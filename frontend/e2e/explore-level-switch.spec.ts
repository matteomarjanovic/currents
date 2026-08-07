import { test, expect, type Page } from '@playwright/test';

// Switching feed levels (General/Personal/New worlds) reuses this route's
// component instance — only `page.params.level` changes — so a debounced
// $effect is what actually refetches. That effect also fires once on the
// component's first run, which used to trigger a redundant, unconditional
// reload ~300ms after every explore page load (see explore-long-press-save
// .spec.ts). Skipping that first run must not break the real case this
// effect exists for: an actual level switch still has to refetch.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const collections = [
	{
		uri: 'at://did:plc:test/is.currents.collection/c1',
		author: { did: me.did, handle: me.handle, displayName: me.displayName },
		name: 'Test Collection',
		previews: [],
		saveCount: 1,
		createdAt: '2026-01-01T00:00:00Z'
	}
];

function makeSave(i: number) {
	return {
		uri: `at://did:plc:test/is.currents.save/s${i}`,
		author: { did: me.did, handle: me.handle, displayName: me.displayName },
		content: {
			$type: 'is.currents.content.defs#imageView',
			blobCid: `bafy${i}`,
			imageUrl: `${APPVIEW}/img/${me.did}/bafy${i}?w=400&h=500`,
			width: 400,
			height: 500,
			alt: `general-tile-${i}`
		},
		originUrl: 'https://example.com/pin',
		createdAt: '2026-01-01T00:00:00Z',
		viewer: { saves: [] }
	};
}

function makePersonalSave(i: number) {
	const s = makeSave(i);
	s.content.alt = `personal-tile-${i}`;
	return s;
}

const generalFeed = [makeSave(1)];
const personalFeed = [makePersonalSave(2)];

async function mockApi(page: Page) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/img/')) {
			return route.fulfill({
				status: 200,
				contentType: 'image/svg+xml',
				body: `<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"><rect width="100%" height="100%" fill="#3b82f6"/></svg>`
			});
		}
		if (url.includes('/api/me')) return json(me);
		if (url.includes('getActorCollections')) return json({ collections });
		if (url.includes('getFeed')) {
			const personalized = new URL(url).searchParams.get('personalized');
			const feed = personalized === '1' ? personalFeed : generalFeed;
			return json({ feed, cursor: null });
		}
		if (url.includes('getSaves')) return json({ saves: [] });
		if (url.includes('getRelatedSaves')) return json({ saves: [] });
		if (url.includes('getImageCollections')) return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

test('switching to Personal refetches and shows the personal feed', async ({ page }) => {
	await mockApi(page);
	await page.goto('/explore/general');
	await page.waitForSelector('a.block img', { timeout: 10_000 });
	await expect(page.locator('img[alt="general-tile-1"]')).toBeVisible();

	await page.getByRole('button', { name: 'Adjust personalization' }).click();
	await page.getByRole('menuitemradio', { name: 'Personal' }).click();

	await expect(page).toHaveURL(/\/explore\/personal/);
	await expect(page.locator('img[alt="personal-tile-2"]')).toBeVisible();
	await expect(page.locator('img[alt="general-tile-1"]')).toHaveCount(0);
});
