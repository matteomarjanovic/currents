import { test, expect, type Page } from '@playwright/test';

// Balanced masonry assigns each laid-out frame a CSS order. Newly appended frames
// briefly have the default order before the library's MutationObserver positions
// them, which can make browser scroll anchoring jump to a different part of the
// feed. Loading another page must leave the viewer at the same scroll offset.

const APPVIEW = 'https://api-dev.currents.is';

function makeSave(i: number) {
	const width = 400;
	const height = [320, 480, 560, 680, 800][i % 5];
	return {
		uri: `at://did:plc:test/is.currents.save/s${i}`,
		author: { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' },
		content: {
			$type: 'is.currents.content.defs#imageView',
			blobCid: `bafy${i}`,
			imageUrl: `${APPVIEW}/img/did:plc:test/bafy${i}`,
			width,
			height,
			alt: `tile-${i}`
		},
		createdAt: '2026-01-01T00:00:00Z',
		viewer: { saves: [] }
	};
}

const firstPage = Array.from({ length: 20 }, (_, i) => makeSave(i));
const secondPage = Array.from({ length: 20 }, (_, i) => makeSave(i + firstPage.length));

async function mockApi(page: Page, waitForSecondPage: Promise<void>) {
	await page.route(`${APPVIEW}/**`, async (route) => {
		const url = new URL(route.request().url());
		const json = (value: unknown) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(value)
			});

		if (url.pathname.includes('/img/')) {
			return route.fulfill({
				status: 200,
				contentType: 'image/svg+xml',
				body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500" />'
			});
		}
		if (url.pathname === '/api/me') return json(null);
		if (url.pathname.includes('getFeed')) {
			if (url.searchParams.has('cursor')) {
				await waitForSecondPage;
				return json({ feed: secondPage, cursor: null });
			}
			return json({ feed: firstPage, cursor: 'next' });
		}
		if (url.pathname.includes('features/seen')) return json({ seen: [] });
		if (url.pathname.includes('moderation/prefs'))
			return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

test('appending a masonry page preserves the scroll position', async ({ page }) => {
	let releaseSecondPage!: () => void;
	const waitForSecondPage = new Promise<void>((resolve) => (releaseSecondPage = resolve));
	await mockApi(page, waitForSecondPage);
	await page.goto('/explore/general');
	await expect(page.locator('img[alt="tile-19"]')).toBeVisible();

	const grid = page.locator('div[style*="grid-template-columns"]').first();
	await grid.evaluate((element) => {
		const sentinel = element.parentElement?.nextElementSibling;
		if (!sentinel) throw new Error('missing infinite-scroll sentinel');
		const sentinelTop = sentinel.getBoundingClientRect().top + window.scrollY;
		window.scrollTo(0, sentinelTop - window.innerHeight - 200);
	});
	await expect(grid.locator('[data-slot="skeleton"]')).toHaveCount(2);

	const before = await page.evaluate(() => window.scrollY);
	releaseSecondPage();
	await expect(page.locator('img[alt="tile-39"]')).toBeVisible();
	await expect(grid.locator('[data-slot="skeleton"]')).toHaveCount(0);
	await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())));

	const after = await page.evaluate(() => window.scrollY);
	expect(after).toBeCloseTo(before, 0);
});
