import { expect, test, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:restore', handle: 'restore.test', displayName: 'Restore' };
const collection = {
	uri: `at://${me.did}/is.currents.feed.collection/ideas`,
	name: 'Ideas',
	author: me,
	saveCount: 1,
	createdAt: '2026-01-01T00:00:00Z'
};
const imageUrl =
	'data:image/svg+xml,' +
	encodeURIComponent(
		'<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"><rect width="400" height="500" fill="steelblue"/></svg>'
	);

function image(level: string, seed: number, i: number) {
	const key = `${level}-${seed}-${i}`;
	return {
		uri: `at://${me.did}/is.currents.feed.save/${key}`,
		author: me,
		createdAt: '2026-01-01T00:00:00Z',
		content: {
			$type: 'is.currents.content.defs#imageView',
			blobCid: key,
			imageUrl,
			width: 400,
			height: 500,
			alt: key
		},
		viewer: { saves: [] }
	};
}

async function mockApi(page: Page, { expire = false, delaySecondPage = false } = {}) {
	const calls: { level: string; cursor: string | null }[] = [];
	const seeds = new Map<string, number>();
	let releaseSecondPage = () => {};
	const secondPage = new Promise<void>((resolve) => {
		releaseSecondPage = resolve;
	});
	await page.route(`${APPVIEW}/**`, async (route) => {
		const url = new URL(route.request().url());
		const path = url.pathname;
		const json = (value: unknown, status = 200) => route.fulfill({ status, json: value });
		if (path.endsWith('getFeed')) {
			const level = url.searchParams.get('personalized')!;
			const cursor = url.searchParams.get('cursor');
			calls.push({ level, cursor });
			if (cursor && expire) return json({ error: 'ExpiredCursor' }, 400);
			if (cursor && delaySecondPage) await secondPage;
			const seed = cursor ? Number(cursor.split(':')[1]) : (seeds.get(level) ?? 0) + 1;
			seeds.set(level, seed);
			return json({
				feed: Array.from({ length: cursor ? 20 : 50 }, (_, i) =>
					image(level, seed, i + (cursor ? 50 : 0))
				),
				cursor: cursor ? undefined : `${level}:${seed}:next`
			});
		}
		if (path === '/api/me' || path.endsWith('getProfile')) return json(me);
		if (path.endsWith('getActorCollections')) return json({ collections: [collection] });
		if (path.endsWith('getImageCollections')) return json({ collections: [collection] });
		if (path.includes('features/seen')) return json({ seen: ['organize-mode'] });
		if (path === '/api/feed/preferences')
			return json({ excludedCollections: [], defaultFeed: 'personal' });
		return json({ saves: [], collections: [] });
	});
	return { calls, releaseSecondPage };
}

async function position(page: Page, y = 900) {
	await expect(page.locator('[data-save-uri]').first()).toBeVisible();
	await page.evaluate((y) => window.scrollTo(0, y), y);
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(y);
	return page.locator('[data-save-uri]').evaluateAll((elements) => {
		const visible = elements
			.map((element) => ({
				uri: element.getAttribute('data-save-uri')!,
				rect: element.getBoundingClientRect()
			}))
			.filter(({ rect }) => rect.top >= 80 && rect.bottom < innerHeight)
			.sort((a, b) => a.rect.top - b.rect.top || a.rect.left - b.rect.left)[0];
		return { uri: visible.uri, top: visible.rect.top, y: scrollY };
	});
}

async function expectPosition(page: Page, saved: Awaited<ReturnType<typeof position>>) {
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBeCloseTo(saved.y, 0);
	await expect
		.poll(() =>
			page
				.locator(`[data-save-uri="${saved.uri}"]`)
				.evaluate((element) => element.getBoundingClientRect().top)
		)
		.toBeCloseTo(saved.top, 0);
}

async function switchLevel(page: Page, name: string) {
	await page.getByRole('button', { name: 'Adjust personalization' }).click();
	await page.getByRole('menuitemradio', { name, exact: true }).click();
}

async function recordPageHandoffs(page: Page, outgoingSelector: string) {
	await page.evaluate((selector) => {
		const start = document.startViewTransition.bind(document);
		const from = location.pathname;
		const outgoing = document.querySelector(selector);
		const handoffs: {
			from: string;
			sourceRetained: boolean;
			ready?: { oldOpacity: string; newOpacity: string };
			finished?: { path: string; y: number };
		}[] = [];
		const record = () => {
			document.documentElement.dataset.testPageHandoffs = JSON.stringify(handoffs);
		};
		document.startViewTransition = (update) => {
			const handoff: (typeof handoffs)[number] = {
				from,
				sourceRetained: outgoing?.isConnected === true
			};
			handoffs.push(handoff);
			const transition = start(update);
			transition.ready.then(
				() => {
					handoff.ready = {
						oldOpacity: getComputedStyle(document.documentElement, '::view-transition-old(root)')
							.opacity,
						newOpacity: getComputedStyle(document.documentElement, '::view-transition-new(root)')
							.opacity
					};
					record();
				},
				() => {}
			);
			transition.finished.then(
				() => {
					handoff.finished = { path: location.pathname, y: scrollY };
					record();
				},
				() => {}
			);
			return transition;
		};
	}, outgoingSelector);
}

async function expectPageHandoff(page: Page, from: string, path: string, y: number) {
	await expect
		.poll(() => page.locator('html').getAttribute('data-test-page-handoffs'))
		.toContain('finished');
	const handoffs = await page.evaluate(() =>
		JSON.parse(document.documentElement.dataset.testPageHandoffs!)
	);
	expect(handoffs).toEqual([
		{
			from,
			sourceRetained: true,
			ready: { oldOpacity: '1', newOpacity: '0' },
			finished: { path, y }
		}
	]);
	await expect(page.locator('html')).not.toHaveAttribute('data-explore-feed-return');
}

test.describe('desktop Explore restoration', () => {
	test.use({ viewport: { width: 1280, height: 800 }, hasTouch: false, isMobile: false });

	test('Organize → Explore resumes the last level, loaded pages, cursor and position', async ({
		page
	}) => {
		const { calls } = await mockApi(page);
		await page.goto('/explore/new-worlds');
		const saved = await position(page, 2100);
		await expect.poll(() => calls.length).toBe(2);
		await expect(page.locator('[data-save-uri]')).toHaveCount(70);
		await page.getByRole('tab', { name: 'Organize', exact: true }).click();
		await expect(page).toHaveURL('/organize');
		await recordPageHandoffs(page, '[data-slot="sidebar-wrapper"]');
		await page.getByRole('tab', { name: 'Explore', exact: true }).click();
		await expect(page).toHaveURL('/explore/new-worlds');
		await expectPosition(page, saved);
		await expectPageHandoff(page, '/organize', '/explore/new-worlds', saved.y);
		await expect(page.locator('[data-save-uri]')).toHaveCount(70);
		expect(calls).toEqual([
			{ level: '-1', cursor: null },
			{ level: '-1', cursor: '-1:1:next' }
		]);
	});

	test('Back from a profile keeps its snapshot visible until the cached feed is positioned', async ({
		page
	}) => {
		const { calls } = await mockApi(page);
		await page.goto('/explore/general');
		const saved = await position(page);
		await page.getByRole('button', { name: 'Profile menu' }).click();
		await page.getByRole('menuitem', { name: /Go to profile/ }).click();
		await expect(page).toHaveURL(`/profile/${me.handle}`);
		await recordPageHandoffs(page, 'h1');
		await page.goBack();
		await expect(page).toHaveURL('/explore/general');
		await expectPosition(page, saved);
		await expectPageHandoff(page, `/profile/${me.handle}`, '/explore/general', saved.y);
		expect(calls).toEqual([{ level: '0', cursor: null }]);
	});

	test('same-level navigation does not wait for a grid remount', async ({ page }) => {
		const { calls } = await mockApi(page);
		await page.goto('/explore/general');
		await position(page);
		await recordPageHandoffs(page, 'main');
		await page.evaluate(() => {
			const link = document.createElement('a');
			link.href = '/explore/general?view=same';
			link.textContent = 'Same feed';
			link.style.cssText = 'position:fixed;top:80px;left:20px;z-index:100';
			document.body.append(link);
		});
		await page.getByRole('link', { name: 'Same feed', exact: true }).click();
		await expect(page).toHaveURL('/explore/general?view=same');
		await page.evaluate(
			() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))
		);
		await expect(page.locator('html')).not.toHaveAttribute('data-explore-feed-return');
		await expect(page.locator('html')).not.toHaveAttribute('data-test-page-handoffs');
		expect(calls).toEqual([{ level: '0', cursor: null }]);
	});

	test('Back through a profile and image restores the feed without refetching', async ({
		page
	}) => {
		const { calls } = await mockApi(page);
		await page.goto('/explore/general');
		// A scroll lock can reflow the background grid when its scrollbar disappears.
		await page.addStyleTag({
			content: 'body.scroll-locked main { padding-top: 34px !important; }'
		});
		const saved = await position(page);
		await page.locator(`[data-save-uri="${saved.uri}"] a.block`).click();
		await page
			.getByRole('button', { name: 'In other collections' })
			.filter({ visible: true })
			.click();
		await page
			.locator(`[data-save-detail-overlay] a[href="/profile/${me.handle}"]`)
			.filter({ visible: true })
			.click();
		await expect(page).toHaveURL(`/profile/${me.handle}`);
		await page.goBack();
		await expect(page.locator('[data-save-detail-overlay]')).toHaveCount(1);
		await page.goBack();
		await expect(page).toHaveURL('/explore/general');
		await expectPosition(page, saved);
		expect(calls).toEqual([{ level: '0', cursor: null }]);
	});

	for (const back of ['browser', 'image button'] as const) {
		test(`${back} Back reapplies the feed anchor after a late scroll-to-top`, async ({ page }) => {
			await mockApi(page);
			await page.goto('/explore/general');
			const saved = await position(page);
			await page.locator(`[data-save-uri="${saved.uri}"] a.block`).click();
			await expect(page.locator('body')).toHaveClass(/scroll-locked/);
			// Model a browser restoring the locked page's scrollY=0 after unlocking.
			await page.evaluate(() => {
				window.addEventListener(
					'popstate',
					() => {
						const samples: number[] = [];
						const closingOverflows: string[] = [];
						let frames = 0;
						const samplePaint = () =>
							requestAnimationFrame(() =>
								setTimeout(() => {
									const overlay = document.querySelector('[data-save-detail-overlay]');
									if (!overlay) samples.push(scrollY);
									else if (!document.body.classList.contains('scroll-locked')) {
										closingOverflows.push(getComputedStyle(overlay).overflowY);
									}
									if (++frames < 4) samplePaint();
									else {
										document.documentElement.dataset.testPaintSamples = JSON.stringify(samples);
										document.documentElement.dataset.testClosingOverflows =
											JSON.stringify(closingOverflows);
									}
								}, 0)
							);
						requestAnimationFrame(() => {
							window.scrollTo(0, 0);
							document.documentElement.dataset.testScrollReset = 'applied';
						});
						samplePaint();
					},
					{ once: true }
				);
			});
			if (back === 'browser') await page.goBack();
			else
				await page
					.getByRole('button', { name: 'Go back', exact: true })
					.filter({ visible: true })
					.first()
					.click();
			await expect(page).toHaveURL('/explore/general');
			await expect(page.locator('html')).toHaveAttribute('data-test-scroll-reset', 'applied');
			await expectPosition(page, saved);
			await expect(page.locator('html')).toHaveAttribute('data-test-paint-samples', /\[/);
			const samples = await page.evaluate(
				() => JSON.parse(document.documentElement.dataset.testPaintSamples!) as number[]
			);
			expect(samples.length).toBeGreaterThan(0);
			expect(samples.every((y) => Math.abs(y - saved.y) < 1)).toBe(true);
			const closingOverflows = await page.evaluate(
				() => JSON.parse(document.documentElement.dataset.testClosingOverflows!) as string[]
			);
			expect(closingOverflows.length).toBeGreaterThan(0);
			expect(closingOverflows.every((overflow) => overflow === 'hidden')).toBe(true);
		});
	}

	test('a restored feed continues from its existing cursor', async ({ page }) => {
		const { calls } = await mockApi(page);
		await page.goto('/explore/general');
		const saved = await position(page);
		await page.getByRole('tab', { name: 'Organize', exact: true }).click();
		await page.getByRole('tab', { name: 'Explore', exact: true }).click();
		await expect(page).toHaveURL('/explore/general');
		await expect(page.locator('[data-save-uri]')).toHaveCount(50);
		await expectPosition(page, saved);
		await position(page, 2100);
		await expect(page.locator('[data-save-uri]')).toHaveCount(70);
		expect(calls).toEqual([
			{ level: '0', cursor: null },
			{ level: '0', cursor: '0:1:next' }
		]);
	});

	test('restores an image anchor after the viewport width changes', async ({ page }) => {
		await mockApi(page);
		await page.goto('/explore/general');
		await position(page);
		const anchor = await page.locator('[data-save-uri]').evaluateAll(
			(elements) =>
				elements
					.map((element) => ({
						uri: element.getAttribute('data-save-uri')!,
						rect: element.getBoundingClientRect()
					}))
					.filter(({ rect }) => rect.bottom > 0 && rect.top < innerHeight)
					.sort((a, b) => a.rect.top - b.rect.top || a.rect.left - b.rect.left)
					.map(({ uri, rect }) => ({ uri, top: rect.top }))[0]
		);
		await page.getByRole('tab', { name: 'Organize', exact: true }).click();
		await page.setViewportSize({ width: 1000, height: 800 });
		await page.getByRole('tab', { name: 'Explore', exact: true }).click();
		await expect
			.poll(() =>
				page
					.locator(`[data-save-uri="${anchor.uri}"]`)
					.evaluate((element) => element.getBoundingClientRect().top)
			)
			.toBeCloseTo(anchor.top, 0);
	});

	test('each feed keeps its own results and position', async ({ page }) => {
		const { calls } = await mockApi(page);
		await page.goto('/explore/general');
		const general = await position(page);
		await switchLevel(page, 'Personal');
		await expect(page.locator('[data-save-uri]').first()).toHaveAttribute(
			'data-save-uri',
			image('1', 1, 0).uri
		);
		const personal = await position(page, 1100);
		await switchLevel(page, 'General');
		await expectPosition(page, general);
		await switchLevel(page, 'Personal');
		await expectPosition(page, personal);
		expect(calls.filter((call) => !call.cursor)).toEqual([
			{ level: '0', cursor: null },
			{ level: '1', cursor: null }
		]);
	});

	test('an expired cursor preserves loaded images until explicit Refresh; reload is fresh', async ({
		page
	}) => {
		const { calls } = await mockApi(page, { expire: true });
		await page.goto('/explore/general');
		await position(page, 2100);
		await expect(page.getByRole('button', { name: 'Refresh feed' })).toBeVisible();
		await expect(page.locator('[data-save-uri]')).toHaveCount(50);
		await page.getByRole('button', { name: 'Refresh feed' }).click();
		await expect(page.locator('[data-save-uri]').first()).toHaveAttribute(
			'data-save-uri',
			image('0', 2, 0).uri
		);
		await expect.poll(() => page.evaluate(() => scrollY)).toBe(0);
		await page.reload();
		await expect(page.locator('[data-save-uri]').first()).toHaveAttribute(
			'data-save-uri',
			image('0', 3, 0).uri
		);
		expect(calls.filter((call) => !call.cursor)).toHaveLength(3);
	});

	test('a late page response cannot mix results after switching levels', async ({ page }) => {
		const { calls, releaseSecondPage } = await mockApi(page, { delaySecondPage: true });
		await page.goto('/explore/general');
		await position(page, 2100);
		await expect.poll(() => calls.length).toBe(2);
		await switchLevel(page, 'Personal');
		await expect(page.locator('[data-save-uri]').first()).toHaveAttribute(
			'data-save-uri',
			image('1', 1, 0).uri
		);
		const response = page.waitForResponse(
			(response) =>
				response.url().includes('getFeed') && new URL(response.url()).searchParams.has('cursor')
		);
		releaseSecondPage();
		await (await response).finished();
		await page.evaluate(
			() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))
		);
		await expect(page.locator('[data-save-uri]')).toHaveCount(50);
		await expect(
			page.locator('[data-save-uri^="at://did:plc:restore/is.currents.feed.save/0-"]')
		).toHaveCount(0);
	});
});

test('mobile mode menu resumes Explore at the previous image', async ({ page }) => {
	const { calls } = await mockApi(page);
	await page.goto('/explore/personal');
	const saved = await position(page);
	await page.getByRole('button', { name: 'Switch mode' }).click();
	await page.getByRole('menuitem', { name: /Organize/ }).click();
	await expect(page).toHaveURL('/organize');
	await page.getByRole('button', { name: 'Switch mode' }).click();
	await page.getByRole('menuitem', { name: /Explore/ }).click();
	await expect(page).toHaveURL('/explore/personal');
	await expectPosition(page, saved);
	expect(calls).toEqual([{ level: '1', cursor: null }]);
});
