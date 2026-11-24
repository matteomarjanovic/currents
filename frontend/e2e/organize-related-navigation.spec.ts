import { test, expect, type Page } from '@playwright/test';

const API = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social' };
const collectionUri = `at://${me.did}/is.currents.feed.collection/test`;
const uri = (key: string) => `at://${me.did}/is.currents.feed.save/${key}`;
function image(key: string) {
	return {
		uri: uri(key),
		author: me,
		createdAt: '2026-01-01T00:00:00Z',
		content: {
			$type: 'is.currents.content.defs#imageView',
			imageUrl: `https://cdn.currents.is/img/${me.did}/${key}`,
			blobCid: key,
			width: 400,
			height: 500,
			alt: `Image ${key}`
		},
		viewer: { saves: [] }
	};
}

async function mockApi(page: Page) {
	const calls: { key: string; cursor: string | null; limit: string | null }[] = [];
	await page.addInitScript(
		(destination) => localStorage.setItem('lastUsedCollectionUri', destination),
		collectionUri
	);
	await page.context().route('https://cdn.currents.is/img/**', (route) =>
		route.fulfill({
			contentType: 'image/svg+xml',
			body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"><rect width="400" height="500" fill="#9cafa3"/><circle cx="200" cy="220" r="100" fill="#e5d8c1"/></svg>'
		})
	);
	await page.context().route(/^https:\/\/(?:api(?:-dev)?\.)?currents\.is\//, (route) => {
		const url = new URL(route.request().url());
		const json = (value: unknown) => route.fulfill({ json: value });
		if (url.pathname === '/api/me') return json(me);
		if (url.pathname === '/api/me/role') return json({ role: 'user' });
		if (url.pathname.includes('getActorCollections'))
			return json({ collections: [{ uri: collectionUri, name: 'Test collection', saveCount: 2 }] });
		if (url.pathname.includes('getFavouriteCollections')) return json({ collections: [] });
		if (url.pathname.includes('features/seen')) return json({ seen: [] });
		if (url.pathname.includes('moderation/prefs'))
			return json({ adult: 'show', aiGenerated: 'show' });
		if (url.pathname === '/api/preferences') return json({});
		if (url.pathname === '/api/supporter/status') return json({ active: true, subscribed: true });
		if (url.pathname.includes('getLibrarySaves'))
			return json({ saves: [image('root'), image('other')] });
		if (url.pathname.includes('getRelatedSaves')) {
			const key = url.searchParams.get('uri')!.split('/').pop()!;
			const cursor = url.searchParams.get('cursor');
			calls.push({ key, cursor, limit: url.searchParams.get('limit') });
			return json(
				cursor
					? { saves: Array.from({ length: 20 }, (_, n) => image(`${key}-more-${n}`)) }
					: {
							saves: ['a', 'b', ...Array.from({ length: 10 }, (_, n) => String(n))].map((suffix) =>
								image(`${key}-${suffix}`)
							),
							cursor: `${key}-more`
						}
			);
		}
		if (url.pathname.includes('getSaves')) return json({ saves: [image('root')] });
		if (url.pathname.endsWith('/resave')) return json({ uri: uri('new-save') });
		return json({ saves: [] });
	});
	return calls;
}

async function openRelated(page: Page, width = 1280) {
	await page.setViewportSize({ width, height: 850 });
	await page.goto('/organize');
	await page.locator(`[data-uri="${uri('root')}"]`).click();
	await page.getByRole('tab', { name: 'Related' }).click();
	const panel = page.getByRole('tabpanel', { name: 'Related' });
	await expect(panel.locator('[data-save-uri]')).toHaveCount(12);
	return panel;
}

for (const width of [1280, 393]) {
	test(`Related navigates inside Organize with compact hero, history, and image arrows at width ${width}`, async ({
		page
	}) => {
		await mockApi(page);
		const panel = await openRelated(page, width);
		const current = panel.locator('[data-related-current]');
		await expect(current).toHaveAttribute('data-related-current', uri('root'));
		expect(
			(await current.locator('[data-swipe-current] img').boundingBox())!.height
		).toBeLessThanOrEqual(193);
		await expect(panel.getByRole('button', { name: 'Back in related images' })).toBeDisabled();
		const actions = (await current.getByRole('button', { name: 'Image actions' }).boundingBox())!;
		expect(actions.x + actions.width).toBeLessThanOrEqual(width);
		await expect(
			panel.getByRole('group', { name: 'Related navigation' }).getByRole('button')
		).toHaveCount(2);
		await current.getByRole('button', { name: 'View selected image full screen' }).focus();
		await page.keyboard.press('ArrowRight');
		await expect(current).toHaveAttribute('data-related-current', uri('other'));
		await expect(page.locator(`[data-uri="${uri('other')}"]`)).toHaveAttribute(
			'aria-current',
			'true'
		);
		await page.keyboard.press('ArrowLeft');
		await expect(current).toHaveAttribute('data-related-current', uri('root'));
		await panel.locator(`[data-save-uri="${uri('root-a')}"] a`).click();
		await expect(current).toHaveAttribute('data-related-current', uri('root-a'));
		await expect(panel.locator(`[data-save-uri="${uri('root-a-a')}"]`)).toBeAttached();
		await expect(page).toHaveURL('/organize');
		await expect(page.locator('[data-save-detail-overlay]')).toHaveCount(0);
		await page.keyboard.press('ArrowRight');
		await expect(current).toHaveAttribute('data-related-current', uri('root-b'));
		await page.keyboard.press('ArrowLeft');
		await expect(current).toHaveAttribute('data-related-current', uri('root-a'));
		await panel.getByRole('button', { name: 'Back in related images' }).click();
		await expect(current).toHaveAttribute('data-related-current', uri('root'));
		await panel.getByRole('button', { name: 'Forward in related images' }).click();
		await expect(current).toHaveAttribute('data-related-current', uri('root-a'));
	});
}

test('Related restores loaded pages and scroll positions across Back, Forward, and tab switches', async ({
	page
}) => {
	const calls = await mockApi(page);
	const panel = await openRelated(page);
	const scroll = panel.locator('[data-related-scroll]');
	await scroll.evaluate((element) => {
		element.scrollTop = element.scrollHeight;
	});
	await expect(panel.locator('[data-save-uri]')).toHaveCount(32);
	expect(calls.filter((call) => call.key === 'root').map((call) => call.limit)).toEqual([
		'12',
		'50'
	]);
	const frame = panel.locator(`[data-save-uri="${uri('root-more-8')}"]`);
	await frame.scrollIntoViewIfNeeded();
	const savedScroll = await scroll.evaluate((element) => element.scrollTop);
	await frame.locator('a').click();
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root-more-8')
	);
	await expect(panel.locator('[data-save-uri]')).toHaveCount(12);
	await panel.getByRole('button', { name: 'Back in related images' }).click();
	await expect(panel.locator('[data-save-uri]')).toHaveCount(32);
	await expect
		.poll(() => scroll.evaluate((element) => element.scrollTop))
		.toBeCloseTo(savedScroll, 0);
	expect(calls.filter((call) => call.key === 'root')).toHaveLength(2);
	await panel.getByRole('button', { name: 'Forward in related images' }).click();
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root-more-8')
	);
	const childCalls = calls.filter((call) => call.key === 'root-more-8').length;
	await page.getByRole('tab', { name: 'Details', exact: true }).click();
	await expect(panel.locator('[data-save-uri]')).toHaveCount(0);
	await page.getByRole('tab', { name: 'Related', exact: true }).click();
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root-more-8')
	);
	await expect(panel.locator('[data-save-uri]')).toHaveCount(12);
	expect(calls.filter((call) => call.key === 'root-more-8')).toHaveLength(childCalls);
});

test('Related shows a vertical image trail on scroll, expands vertically, and previews to the left', async ({
	page
}) => {
	await mockApi(page);
	const panel = await openRelated(page);
	const scroll = panel.locator('[data-related-scroll]');
	await expect(panel.getByRole('navigation', { name: 'Image trail' })).toHaveCount(0);
	await scroll.evaluate((element) => {
		element.scrollTop = 600;
	});
	const trail = panel.getByRole('navigation', { name: 'Image trail' });
	await expect(trail).toHaveAttribute('data-orientation', 'vertical');
	await trail.getByRole('button', { name: /Return to image 1/ }).click();
	await expect.poll(() => scroll.evaluate((element) => element.scrollTop)).toBe(0);
	await panel.locator(`[data-save-uri="${uri('root-a')}"] a`).click();
	await panel.locator(`[data-save-uri="${uri('root-a-a')}"] a`).click();
	await expect(trail.getByRole('button')).toHaveCount(2);
	await page.mouse.move(1, 1);
	const toolbar = (await panel.locator('[data-related-toolbar]').boundingBox())!;
	const close = (await page.getByRole('button', { name: 'Close details' }).boundingBox())!;
	const trailBox = (await trail.boundingBox())!;
	const gridInset = await scroll.evaluate((element) =>
		parseFloat(getComputedStyle(element).paddingRight)
	);
	expect(toolbar.x + toolbar.width - trailBox.x - trailBox.width).toBeCloseTo(gridInset, 1);
	await expect
		.poll(() =>
			trail.getByRole('button').evaluateAll(
				(buttons, { toolbar, close, trailBox }) =>
					buttons.every((button) => {
						const box = button.getBoundingClientRect();
						return (
							box.top >= close.y + close.height &&
							box.bottom <= toolbar.y + toolbar.height - 5 &&
							Math.abs(box.x + box.width / 2 - (trailBox.x + trailBox.width / 2)) < 1
						);
					}),
				{ toolbar, close, trailBox }
			)
		)
		.toBe(true);
	const stackTop = await trail
		.getByRole('button')
		.evaluateAll((buttons) =>
			Math.min(...buttons.map((button) => button.getBoundingClientRect().top))
		);
	expect(stackTop - (close.y + close.height)).toBeLessThan(10);
	const first = trail.getByRole('button', { name: /Return to image 1/ });
	const collapsed = (await first.boundingBox())!;
	const second = trail.getByRole('button', { name: /Return to image 2/ });
	await second.hover();
	await expect
		.poll(async () => {
			const boxes = await trail
				.getByRole('button')
				.evaluateAll((elements) => elements.map((element) => element.getBoundingClientRect().y));
			return boxes[1] - boxes[0];
		})
		.toBeGreaterThan(45);
	await expect
		.poll(async () => {
			const expanded = (await first.boundingBox())!;
			return Math.abs(expanded.y + expanded.height / 2 - collapsed.y - collapsed.height / 2);
		})
		.toBeLessThan(5);
	await second.hover();
	const preview = page.locator(`[data-trail-preview="${uri('root-a')}"]`);
	await expect(preview).toBeVisible();
	const previewBox = (await preview.boundingBox())!;
	expect(previewBox.x + previewBox.width).toBeLessThan((await second.boundingBox())!.x);
	await second.click();
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root-a')
	);
	await expect(panel.getByRole('button', { name: 'Forward in related images' })).toBeEnabled();
});

test('Related keyboard arrows navigate siblings without interfering with menus or the full-screen viewer', async ({
	page
}) => {
	await mockApi(page);
	const panel = await openRelated(page);
	await panel.locator(`[data-save-uri="${uri('root-a')}"] a`).click();
	const current = panel.locator('[data-related-current]');
	await current.getByRole('button', { name: 'Test collection', exact: true }).click();
	await page.keyboard.press('ArrowRight');
	await expect(current).toHaveAttribute('data-related-current', uri('root-a'));
	await page.keyboard.press('Escape');
	await expect(page.locator('[data-slot="popover-content"]')).toHaveCount(0);
	await page.keyboard.press('ArrowRight');
	await expect(current).toHaveAttribute('data-related-current', uri('root-b'));
	await current.getByRole('button', { name: 'View selected image full screen' }).click();
	await page.keyboard.press('ArrowLeft');
	await expect(current).toHaveAttribute('data-related-current', uri('root-b'));
	await page.getByRole('button', { name: 'Close image viewer' }).click();
	await expect(page.getByRole('dialog', { name: 'Image viewer' })).toHaveCount(0);
	await page.keyboard.press('ArrowLeft');
	await expect(current).toHaveAttribute('data-related-current', uri('root-a'));
});

test('a new canvas selection resets the Related journey', async ({ page }) => {
	await mockApi(page);
	const panel = await openRelated(page);
	await panel.locator(`[data-save-uri="${uri('root-a')}"] a`).click();
	await page.locator(`[data-uri="${uri('other')}"]`).click();
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('other')
	);
	await expect(panel.getByRole('button', { name: 'Back in related images' })).toBeDisabled();
	await expect(panel.getByRole('button', { name: 'Forward in related images' })).toBeDisabled();
});

test('a deep Related journey retains data with only one hero and grid mounted', async ({
	page
}) => {
	const calls = await mockApi(page);
	const panel = await openRelated(page);
	let key = 'root';
	for (let depth = 0; depth < 12; depth++) {
		key += '-a';
		await panel.locator(`[data-save-uri="${uri(key)}"] a`).click();
		await expect(panel.locator('[data-related-current]')).toHaveAttribute(
			'data-related-current',
			uri(key)
		);
		await expect(panel.locator('[data-save-uri]')).toHaveCount(12);
		await expect(panel.locator('[data-related-current]')).toHaveCount(1);
	}
	const root = panel
		.getByRole('navigation', { name: 'Image trail' })
		.getByRole('button', { name: /Return to image 1:/ });
	await root.focus();
	await root.press('Enter');
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root')
	);
	await expect(panel.locator('[data-save-uri]')).toHaveCount(12);
	expect(calls.filter((call) => call.key === 'root')).toHaveLength(1);
	await panel.locator(`[data-save-uri="${uri('root-b')}"] a`).click();
	await expect(panel.getByRole('button', { name: 'Forward in related images' })).toBeDisabled();
});

test('a late related response cannot replace the next image or its results', async ({ page }) => {
	await mockApi(page);
	let release!: () => void;
	const waiting = new Promise<void>((resolve) => {
		release = resolve;
	});
	await page.route(`${API}/xrpc/is.currents.feed.getRelatedSaves?**`, async (route) => {
		if (new URL(route.request().url()).searchParams.get('uri') !== uri('root-a'))
			return route.fallback();
		await waiting;
		return route.fulfill({ json: { saves: [image('stale')] } });
	});
	const panel = await openRelated(page);
	await panel.locator(`[data-save-uri="${uri('root-a')}"] a`).click();
	await page.keyboard.press('ArrowRight');
	await expect(panel.locator(`[data-save-uri="${uri('root-b-a')}"]`)).toBeAttached();
	const response = page.waitForResponse(
		(response) => new URL(response.url()).searchParams.get('uri') === uri('root-a')
	);
	release();
	await (await response).finished();
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root-b')
	);
	await expect(panel.locator(`[data-save-uri="${uri('stale')}"]`)).toHaveCount(0);
});

test('saving the selected related image updates its cached parent card', async ({ page }) => {
	await mockApi(page);
	const panel = await openRelated(page);
	await panel.locator(`[data-save-uri="${uri('root-a')}"] a`).click();
	const current = panel.locator('[data-related-current]');
	await current.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(current.getByRole('button', { name: 'Saved', exact: true })).toBeVisible();
	await panel.getByRole('button', { name: 'Back in related images' }).click();
	const card = panel.locator(`[data-save-uri="${uri('root-a')}"]`);
	await card.hover();
	await expect(card.getByRole('button', { name: 'Saved', exact: true })).toBeVisible();
});

test('Related shows the existing saved collection instead of the last-used collection', async ({
	page
}) => {
	await mockApi(page);
	const existing = `at://${me.did}/is.currents.feed.collection/existing`;
	await page.route(`${API}/xrpc/is.currents.feed.getActorCollections?**`, (route) =>
		route.fulfill({
			json: {
				collections: [
					{ uri: collectionUri, name: 'Test collection' },
					{ uri: existing, name: 'Existing collection' }
				]
			}
		})
	);
	await page.route(`${API}/xrpc/is.currents.feed.getLibrarySaves?**`, (route) =>
		route.fulfill({
			json: {
				saves: [
					{
						...image('root'),
						viewer: { saves: [{ collectionUri: existing, saveUri: uri('saved-root') }] }
					},
					image('other')
				]
			}
		})
	);
	const panel = await openRelated(page);
	const controls = panel.locator('[data-related-current]');
	await expect(
		controls.getByRole('button', { name: 'Existing collection', exact: true })
	).toBeVisible();
	await expect(controls.getByRole('button', { name: 'Saved', exact: true })).toBeEnabled();
	await panel.locator(`[data-save-uri="${uri('root-a')}"] a`).click();
	await expect(controls.getByRole('button', { name: 'Save', exact: true })).toBeEnabled();
	await expect(
		controls.getByRole('button', { name: 'Test collection', exact: true })
	).toBeVisible();
});

test('image arrows keep a decoded image visible when two records share its URL', async ({
	page
}) => {
	await mockApi(page);
	await page.route(`${API}/xrpc/is.currents.feed.getLibrarySaves?**`, (route) =>
		route.fulfill({
			json: { saves: [image('root'), { ...image('other'), content: image('root').content }] }
		})
	);
	const panel = await openRelated(page);
	const hero = panel.locator('[data-related-current] [data-swipe-current] img');
	await expect(hero).toHaveCSS('opacity', '1');
	await panel.getByRole('button', { name: 'View selected image full screen' }).focus();
	await page.keyboard.press('ArrowRight');
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('other')
	);
	await expect(hero).toHaveCSS('opacity', '1');
});

test('widening Related loads the next page when the short first page is near the viewport', async ({
	page
}) => {
	const calls = await mockApi(page);
	const panel = await openRelated(page);
	const handle = page.getByRole('separator', { name: 'Resize details sidebar' });
	await expect
		.poll(async () => Number(await handle.getAttribute('aria-valuemax')))
		.toBeGreaterThan(400);
	await handle.focus();
	await page.keyboard.press('End');
	await expect(panel.locator('[data-save-uri]')).toHaveCount(32);
	expect(calls.filter((call) => call.key === 'root').map((call) => call.limit)).toEqual([
		'12',
		'50'
	]);
});

test('resizing Details keeps the top-left visible main-grid tile at its original vertical offset', async ({
	page
}) => {
	await mockApi(page);
	await page.route(`${API}/xrpc/is.currents.feed.getLibrarySaves?**`, (route) =>
		route.fulfill({ json: { saves: Array.from({ length: 64 }, (_, i) => image(`tile-${i}`)) } })
	);
	await page.setViewportSize({ width: 1440, height: 850 });
	await page.goto('/organize');
	const grid = page.locator('[data-organize-scroll]');
	await grid.locator('[data-uri]').first().click();
	const handle = page.getByRole('separator', { name: 'Resize details sidebar' });
	const panel = page.locator(`[id="${await handle.getAttribute('aria-controls')}"]`);
	await expect(panel).toHaveCSS('width', '352px');
	// Let the open animation and its clicked-tile pin finish before scrolling.
	await page.waitForTimeout(350);
	await grid.evaluate((element) => {
		element.scrollTop = 1100;
	});
	const getAnchor = () =>
		grid.evaluate((element) => {
			const viewport = element.getBoundingClientRect();
			const tiles = Array.from(element.querySelectorAll<HTMLElement>('[data-uri]'), (tile) => {
				const { top, left } = tile.getBoundingClientRect();
				return { uri: tile.dataset.uri, top: top - viewport.top, left };
			});
			return tiles
				.filter(({ top }) => top >= -1 && top < viewport.height)
				.sort((a, b) => (Math.abs(a.top - b.top) <= 1 ? a.left - b.left : a.top - b.top))[0];
		});
	const offsetError = (anchor: Awaited<ReturnType<typeof getAnchor>>) =>
		grid
			.locator(`[data-uri="${anchor.uri}"]`)
			.evaluate(
				(tile, top) =>
					Math.abs(
						tile.getBoundingClientRect().top -
							tile.closest('[data-organize-scroll]')!.getBoundingClientRect().top -
							top
					),
				anchor.top
			);
	const pointerAnchor = await getAnchor();
	const grip = (await handle.boundingBox())!;
	const x = grip.x + grip.width / 2;
	const y = grip.y + grip.height / 2;
	await page.mouse.move(x, y);
	await page.mouse.down();
	await page.mouse.move(x - 210, y, { steps: 8 });
	await expect(panel).toHaveCSS('width', '562px');
	await expect
		.poll(() => grid.evaluate((element) => Math.abs(element.scrollTop - 1100)))
		.toBeGreaterThan(100);
	await expect.poll(() => offsetError(pointerAnchor)).toBeLessThan(2);
	await page.mouse.move(x - 400, y, { steps: 8 });
	await page.mouse.up();
	await expect(panel).toHaveCSS('width', '752px');
	await expect.poll(() => offsetError(pointerAnchor)).toBeLessThan(2);

	const keyboardAnchor = await getAnchor();
	await handle.press('Home');
	await expect(panel).toHaveCSS('width', '352px');
	await expect.poll(() => offsetError(keyboardAnchor)).toBeLessThan(2);
});

test('Details keyboard arrows change the actual main-grid selection and reveal offscreen images', async ({
	page
}) => {
	await mockApi(page);
	const saves = Array.from({ length: 18 }, (_, n) => image(`library-${n}`));
	await page.route(`${API}/xrpc/is.currents.feed.getLibrarySaves?**`, (route) =>
		route.fulfill({ json: { saves } })
	);
	await page.setViewportSize({ width: 1280, height: 850 });
	await page.goto('/organize');
	await page.locator(`[data-uri="${uri('library-0')}"]`).click();
	const details = page.getByRole('tabpanel', { name: 'Details' });
	await details.getByRole('button', { name: 'View image full screen', exact: true }).focus();
	for (let n = 1; n <= 15; n++) {
		await page.keyboard.press('ArrowRight');
		await expect(page.locator(`[data-uri="${uri(`library-${n}`)}"]`)).toHaveAttribute(
			'aria-current',
			'true'
		);
	}
	await expect(page.locator(`[data-uri="${uri('library-15')}"]`)).toBeInViewport({ ratio: 0.9 });
	await expect(
		details.getByRole('button', { name: 'View image full screen', exact: true }).locator('img')
	).toHaveAttribute('alt', 'Image library-15');
	await page.keyboard.press('ArrowLeft');
	await expect(page.locator(`[data-uri="${uri('library-14')}"]`)).toHaveAttribute(
		'aria-current',
		'true'
	);
});

test('nested Related arrows follow the parent related grid and never fall back into main-grid navigation', async ({
	page
}) => {
	await mockApi(page);
	const panel = await openRelated(page);
	const current = panel.locator('[data-related-current]');
	await current.getByRole('button', { name: 'View selected image full screen' }).focus();
	await page.keyboard.press('ArrowRight');
	await expect(page.locator(`[data-uri="${uri('other')}"]`)).toHaveAttribute(
		'aria-current',
		'true'
	);
	await panel.locator(`[data-save-uri="${uri('other-a')}"] a`).click();
	await page.keyboard.press('ArrowLeft');
	await expect(current).toHaveAttribute('data-related-current', uri('other-a'));
	await expect(page.locator(`[data-uri="${uri('other')}"]`)).toHaveAttribute(
		'aria-current',
		'true'
	);
	await page.keyboard.press('ArrowRight');
	await expect(current).toHaveAttribute('data-related-current', uri('other-b'));
	await panel.locator(`[data-save-uri="${uri('other-b-a')}"] a`).click();
	await page.keyboard.press('ArrowRight');
	await expect(current).toHaveAttribute('data-related-current', uri('other-b-b'));
	await panel.getByRole('button', { name: 'Back in related images' }).click();
	await expect(current).toHaveAttribute('data-related-current', uri('other-b'));
	await page.keyboard.press('ArrowRight');
	await expect(current).toHaveAttribute('data-related-current', uri('other-0'));
	await expect(page.locator(`[data-uri="${uri('other')}"]`)).toHaveAttribute(
		'aria-current',
		'true'
	);
});

test('Details arrows still select the main grid after Related browsing and leave editors alone', async ({
	page
}) => {
	await mockApi(page);
	const panel = await openRelated(page);
	await panel.locator(`[data-save-uri="${uri('root-a')}"] a`).click();
	await page.getByRole('tab', { name: 'Details', exact: true }).click();
	const details = page.getByRole('tabpanel', { name: 'Details' });
	await details.getByRole('button', { name: 'Add AT Protocol source' }).click();
	const editor = page.getByRole('dialog', { name: 'AT Protocol source' });
	await editor.getByLabel('Record URI').fill('at://did:plc:test/app.bsky.feed.post/one');
	await page.keyboard.press('ArrowRight');
	await expect(page.locator(`[data-uri="${uri('root')}"]`)).toHaveAttribute('aria-current', 'true');
	await editor.getByRole('button', { name: 'Cancel' }).click();
	await expect(editor).toHaveCount(0);
	await details.getByRole('button', { name: 'View image full screen', exact: true }).focus();
	await page.keyboard.press('ArrowRight');
	await expect(page.locator(`[data-uri="${uri('other')}"]`)).toHaveAttribute(
		'aria-current',
		'true'
	);
	await page.getByRole('tab', { name: 'Related', exact: true }).click();
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('other')
	);
	await expect(panel.getByRole('button', { name: 'Back in related images' })).toBeDisabled();
});

async function swipeImage(
	page: Page,
	panel: ReturnType<Page['getByRole']>,
	dx: number,
	dy = 0,
	startX?: number
) {
	const stage = panel.locator('[data-organize-swipe-stage]');
	const box = (await stage.boundingBox())!;
	const from = { x: startX ?? box.x + box.width / 2, y: box.y + box.height / 2 };
	const client = await page.context().newCDPSession(page);
	await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [from] });
	for (let step = 1; step <= 6; step++) {
		await client.send('Input.dispatchTouchEvent', {
			type: 'touchMove',
			touchPoints: [{ x: from.x + (dx * step) / 6, y: from.y + (dy * step) / 6 }]
		});
	}
	await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
	await expect(stage).toHaveAttribute('data-swiping', 'false');
}

for (const tab of ['Details', 'Related']) {
	test(`mobile ${tab} image swipes select the main grid without opening the viewer`, async ({
		page
	}) => {
		await mockApi(page);
		await page.setViewportSize({ width: 393, height: 850 });
		await page.goto('/organize');
		await page.locator(`[data-uri="${uri('root')}"]`).tap();
		await page.getByRole('tab', { name: tab, exact: true }).tap();
		const panel = page.getByRole('tabpanel', { name: tab });
		await expect(panel.locator('[data-swipe-neighbour="1"] img')).toHaveAttribute(
			'loading',
			'eager'
		);
		await swipeImage(page, panel, -120);
		await expect(page.locator(`[data-uri="${uri('other')}"]`)).toHaveAttribute(
			'aria-current',
			'true'
		);
		if (tab === 'Related')
			await expect(panel.locator('[data-related-current]')).toHaveAttribute(
				'data-related-current',
				uri('other')
			);
		await expect(page.getByRole('dialog', { name: 'Image viewer' })).toHaveCount(0);
		await swipeImage(page, panel, -120);
		await expect(page.locator(`[data-uri="${uri('other')}"]`)).toHaveAttribute(
			'aria-current',
			'true'
		);
		await swipeImage(page, panel, 120);
		await expect(page.locator(`[data-uri="${uri('root')}"]`)).toHaveAttribute(
			'aria-current',
			'true'
		);
		await expect(page).toHaveURL('/organize');
	});

	test(`mobile ${tab} image lets vertical drags scroll and leaves screen-edge gestures alone`, async ({
		page
	}) => {
		await mockApi(page);
		await page.setViewportSize({ width: 393, height: 850 });
		await page.goto('/organize');
		await page.locator(`[data-uri="${uri('root')}"]`).tap();
		await page.getByRole('tab', { name: tab, exact: true }).tap();
		const panel = page.getByRole('tabpanel', { name: tab });
		await swipeImage(page, panel, -120, 0, 10);
		await expect(page.locator(`[data-uri="${uri('root')}"]`)).toHaveAttribute(
			'aria-current',
			'true'
		);
		await swipeImage(page, panel, 0, -140);
		const scroll = tab === 'Related' ? panel.locator('[data-related-scroll]') : panel;
		await expect.poll(() => scroll.evaluate((element) => element.scrollTop)).toBeGreaterThan(20);
		await expect(page.locator(`[data-uri="${uri('root')}"]`)).toHaveAttribute(
			'aria-current',
			'true'
		);
	});
}

test('mobile nested swipes stay in the parent related list and keep Back and Forward without a thumbnail stack', async ({
	page
}) => {
	await mockApi(page);
	const panel = await openRelated(page, 393);
	await panel.locator(`[data-save-uri="${uri('root-a')}"] a`).tap();
	await swipeImage(page, panel, -120);
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root-b')
	);
	await expect(page.locator(`[data-uri="${uri('root')}"]`)).toHaveAttribute('aria-current', 'true');
	await swipeImage(page, panel, 120);
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root-a')
	);
	await swipeImage(page, panel, 120);
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root-a')
	);
	await expect(page.locator('[data-mobile-history-layer]')).toHaveCount(0);
	await expect(panel.getByRole('button', { name: 'Open image history' })).toHaveCount(0);
	const controls = panel.locator('[data-mobile-related-navigation]');
	const back = controls.getByRole('button', { name: 'Back in related images' });
	const box = (await back.boundingBox())!;
	expect(box.width).toBeGreaterThanOrEqual(44);
	expect(box.height).toBeGreaterThanOrEqual(44);
	expect(box.y).toBeGreaterThan(700);
	await back.tap();
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root')
	);
	await controls.getByRole('button', { name: 'Forward in related images' }).tap();
	await expect(panel.locator('[data-related-current]')).toHaveAttribute(
		'data-related-current',
		uri('root-a')
	);
});

async function pinchTopImage(page: Page, panel: ReturnType<Page['getByRole']>, native: boolean) {
	const box = (await panel.locator('[data-organize-swipe-stage]').boundingBox())!;
	const middle = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
	const points = (distance: number) => [
		{ id: 1, x: middle.x - distance / 2, y: middle.y },
		{ id: 2, x: middle.x + distance / 2, y: middle.y }
	];
	const client = await page.context().newCDPSession(page);
	await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: points(60) });
	if (native) await expect(page.locator('[data-image-focus]')).toBeVisible();
	for (let step = 1; step <= 5; step++)
		await client.send('Input.dispatchTouchEvent', {
			type: 'touchMove',
			touchPoints: points(60 + step * 24)
		});
	await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
}

for (const tab of ['Details', 'Related']) {
	for (const native of [false, true]) {
		test(`${native ? 'native' : 'web'} ${tab} pinch preserves zoom and never becomes image navigation`, async ({
			page
		}) => {
			if (native)
				await page.addInitScript(() => {
					Object.defineProperty(window, 'androidBridge', { value: {}, configurable: true });
					Object.assign(window, {
						Capacitor: {
							PluginHeaders: [
								{ name: 'SecurePreferences', methods: [{ name: 'get', rtype: 'promise' }] }
							],
							async nativePromise() {
								return { value: null };
							}
						}
					});
				});
			await mockApi(page);
			await page.setViewportSize({ width: 393, height: 850 });
			await page.goto('/organize');
			await page.locator(`[data-uri="${uri('root')}"]`).tap();
			await page.getByRole('tab', { name: tab, exact: true }).tap();
			const panel = page.getByRole('tabpanel', { name: tab });
			await pinchTopImage(page, panel, native);
			await expect(page.locator(`[data-uri="${uri('root')}"]`)).toHaveAttribute(
				'aria-current',
				'true'
			);
			if (native) {
				await expect
					.poll(async () => {
						const style = await page
							.locator('[data-image-focus] > [role="presentation"]')
							.getAttribute('style');
						return Number(style?.match(/scale\(([^)]+)\)/)?.[1] ?? 0);
					})
					.toBeGreaterThan(2.5);
				await page.getByRole('button', { name: 'Close image viewer' }).tap();
				await expect(page.locator('[data-image-focus]')).toHaveCount(0);
				await swipeImage(page, panel, -120);
				await expect(page.locator(`[data-uri="${uri('other')}"]`)).toHaveAttribute(
					'aria-current',
					'true'
				);
			} else await expect(page.locator('[data-image-focus]')).toHaveCount(0);
		});
	}
}
