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
const authedFirstPage = Array.from({ length: 50 }, (_, i) => makeSave(i));
const responsiveSecondPage = Array.from({ length: 50 }, (_, i) =>
	makeSave(i + authedFirstPage.length)
);

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

async function mockAuthedApi(page: Page, waitForSecondSuggestions: Promise<void>) {
	const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
	const collections = Array.from({ length: 773 }, (_, i) => ({
		uri: `at://${me.did}/is.currents.feed.collection/c${i}`,
		name: `Collection ${i}`,
		createdAt: '2026-01-01T00:00:00Z'
	}));
	const suggestionBatches: string[][] = [];

	await page.addInitScript(
		(uri) => localStorage.setItem('lastUsedCollectionUri', uri),
		collections[0].uri
	);
	await page.route(`${APPVIEW}/**`, async (route) => {
		const request = route.request();
		const url = request.url();
		const json = (value: unknown) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(value)
			});

		if (url.includes('/img/')) {
			return route.fulfill({
				status: 200,
				contentType: 'image/svg+xml',
				body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500" />'
			});
		}
		if (url.includes('/api/save-suggestions')) {
			const uris = (request.postDataJSON() as { saveUris: string[] }).saveUris;
			suggestionBatches.push(uris);
			if (suggestionBatches.length === 2) await waitForSecondSuggestions;
			return json({
				suggestions: Object.fromEntries(uris.map((uri) => [uri, collections[0].uri]))
			});
		}
		if (url.includes('/api/preferences')) {
			return json({
				gifAutoplay: false,
				organizeCollectionSort: 'name',
				saveSuggestionMode: 'recommended-then-last-used'
			});
		}
		if (url.includes('/api/feed/preferences')) {
			return json({ excludedCollections: [], defaultFeed: 'general' });
		}
		if (url.includes('/api/me/role')) return json({ role: 'user' });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('getActorCollections')) return json({ collections });
		if (url.includes('getFeed')) {
			return url.includes('cursor=')
				? json({ feed: responsiveSecondPage, cursor: null })
				: json({ feed: authedFirstPage, cursor: 'next' });
		}
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

	return suggestionBatches;
}

test('appending a masonry page preserves the scroll position', async ({ page }) => {
	let releaseSecondPage!: () => void;
	const waitForSecondPage = new Promise<void>((resolve) => (releaseSecondPage = resolve));
	await mockApi(page, waitForSecondPage);
	await page.goto('/explore/general');
	await expect(page.locator('img[alt="tile-0"]')).toBeVisible();

	const grid = page.locator('div[style*="grid-template-columns"]').first();
	await expect.poll(() => grid.evaluate((element) => element.children.length)).toBe(21);
	await grid.evaluate((element) => {
		const sentinel = element.parentElement?.nextElementSibling;
		if (!sentinel) throw new Error('missing infinite-scroll sentinel');
		const sentinelTop = sentinel.getBoundingClientRect().top + window.scrollY;
		window.scrollTo(0, sentinelTop - window.innerHeight - 200);
	});
	await expect(page.locator('img[alt="tile-19"]')).toBeVisible();
	await expect(grid.locator('[data-slot="skeleton"]')).toHaveCount(2);

	const before = await page.evaluate(() => window.scrollY);
	releaseSecondPage();
	await expect(grid.locator('[data-slot="skeleton"]')).toHaveCount(0);
	await expect.poll(() => grid.evaluate((element) => element.children.length)).toBe(41);
	await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())));

	const after = await page.evaluate(() => window.scrollY);
	expect(after).toBeCloseTo(before, 0);
});

test('a signed-in page append keeps the feed interactive and suggestions batched', async ({
	page
}) => {
	let releaseSecondSuggestions!: () => void;
	const waitForSecondSuggestions = new Promise<void>(
		(resolve) => (releaseSecondSuggestions = resolve)
	);
	const suggestionBatches = await mockAuthedApi(page, waitForSecondSuggestions);
	await page.goto('/explore/general');
	await expect.poll(() => suggestionBatches.length).toBe(1);
	await expect(page.locator('img[alt="tile-0"]')).toBeVisible();
	await expect(page.locator('[data-menu-dismiss-surface]')).toHaveCount(0);
	const grid = page.locator('div[style*="grid-template-columns"]').first();
	await expect.poll(() => grid.evaluate((element) => element.children.length)).toBe(51);
	// Approximate a slower production client. Appending the next page must add all
	// frame geometry without constructing all 50 off-screen interactive cards.
	const cdp = await page.context().newCDPSession(page);
	await cdp.send('Emulation.setCPUThrottlingRate', { rate: 6 });

	await page.evaluate(() => {
		const target = window as typeof window & { frameGaps?: number[] };
		target.frameGaps = [];
		let last = performance.now();
		const tick = (now: number) => {
			target.frameGaps!.push(now - last);
			last = now;
			requestAnimationFrame(tick);
		};
		requestAnimationFrame(tick);
	});

	await grid.evaluate((element) => {
		const sentinel = element.parentElement?.nextElementSibling;
		if (!sentinel) throw new Error('missing infinite-scroll sentinel');
		const sentinelTop = sentinel.getBoundingClientRect().top + window.scrollY;
		window.scrollTo(0, sentinelTop - window.innerHeight - 200);
	});
	await expect.poll(() => suggestionBatches.length).toBe(2);
	const appendFrameGap = await page.evaluate(() => {
		const target = window as typeof window & { frameGaps?: number[] };
		const max = Math.max(0, ...(target.frameGaps ?? []));
		target.frameGaps = [];
		return max;
	});
	await page.waitForTimeout(500);
	const nearbyCardFrameGap = await page.evaluate(() => {
		const target = window as typeof window & { frameGaps?: number[] };
		const max = Math.max(0, ...(target.frameGaps ?? []));
		target.frameGaps = [];
		return max;
	});

	await page.getByLabel('Adjust personalization').click();
	await expect(page.getByRole('menu')).toBeVisible();
	await expect(page.locator('[data-menu-dismiss-surface]')).toHaveCount(1);
	await page.keyboard.press('Escape');
	await expect(page.locator('[data-menu-dismiss-surface]')).toHaveCount(0);
	releaseSecondSuggestions();

	await expect(grid.locator('[data-slot="skeleton"]')).toHaveCount(0);
	await expect.poll(() => grid.evaluate((element) => element.children.length)).toBe(101);
	const mountedCards = await page.locator('a.block img').count();
	expect(mountedCards).toBeGreaterThan(0);
	expect(mountedCards).toBeLessThan(100);
	expect(suggestionBatches.map((batch) => batch.length)).toEqual([50, 50]);
	const interactionFrameGap = await page.evaluate(() =>
		Math.max(0, ...((window as typeof window & { frameGaps?: number[] }).frameGaps ?? []))
	);
	expect(appendFrameGap).toBeLessThan(250);
	expect(nearbyCardFrameGap).toBeLessThan(250);
	expect(interactionFrameGap).toBeLessThan(1000);
});
