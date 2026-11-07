import { test, expect, type Page } from '@playwright/test';

const API = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social' };
const other = { did: 'did:plc:other', handle: 'other.bsky.social' };
const collection = (key: string, name: string, author = me) => ({
	uri: `at://${author.did}/is.currents.collection/${key}`,
	cid: `cid-${key}`,
	name,
	author,
	saveCount: 4,
	previews: [{ url: `${API}/img/${author.did}/${key}` }]
});
const own = collection('interiors', 'Interiors');
const foreign = collection('inspiration', 'Inspiration', other);
const more = collection('more', 'More inspiration', other);
const makeSave = (key: string) => ({
	uri: `at://${me.did}/is.currents.save/${key}`,
	author: me,
	createdAt: '2026-01-01T00:00:00Z',
	content: {
		$type: 'is.currents.content.defs#imageView',
		imageUrl: `${API}/img/${me.did}/${key}`,
		blobCid: `cid-${key}`,
		width: 400,
		height: 500
	},
	viewer: { saves: [{ collectionUri: own.uri, saveUri: `at://${me.did}/is.currents.save/${key}` }] }
});
const saves = [makeSave('s1'), makeSave('s2')];

async function mockApi(page: Page) {
	const calls = { favourites: [] as unknown[], unfavourites: 0 };
	let favourited = false;
	await page.route(`${API}/**`, async (route) => {
		const request = route.request();
		const url = new URL(request.url());
		const json = (value: unknown) => route.fulfill({ json: value });
		if (url.pathname.includes('/img/'))
			return route.fulfill({
				contentType: 'image/svg+xml',
				body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"><rect width="400" height="500" fill="#719a85"/></svg>'
			});
		if (url.pathname === '/api/favourite' && request.method() === 'POST') {
			calls.favourites.push(request.postDataJSON());
			favourited = true;
			return json({ uri: `at://${me.did}/is.currents.favourite/f1` });
		}
		if (url.pathname === '/api/favourite/f1' && request.method() === 'DELETE') {
			calls.unfavourites++;
			favourited = false;
			return route.fulfill({ status: 204 });
		}
		if (url.pathname.includes('/api/me/role')) return json({ role: 'user' });
		if (url.pathname.includes('/api/me')) return json(me);
		if (url.pathname.includes('getActorCollections')) return json({ collections: [own] });
		if (url.pathname.includes('getFavouriteCollections'))
			return json({ collections: favourited ? [foreign] : [] });
		if (url.pathname.includes('getImageCollections'))
			return json(
				url.searchParams.has('cursor')
					? { collections: [more] }
					: { collections: [own, foreign], cursor: 'next' }
			);
		if (url.pathname.includes('getLibrarySaves')) return json({ saves });
		if (url.pathname.includes('getSaves')) return json({ saves });
		if (url.pathname.includes('moderation/prefs'))
			return json({ adult: 'blur', aiGenerated: 'show' });
		if (url.pathname.includes('features/seen')) return json({ seen: [] });
		return json({ saves: [] });
	});
	return calls;
}

test.use({ viewport: { width: 1440, height: 1000 }, hasTouch: false, isMobile: false });

test('collection chips have equal heights and previews above, with favourite actions for other collections', async ({
	page
}, testInfo) => {
	const calls = await mockApi(page);
	await page.goto('/organize');
	await page.locator('[data-uri]').first().click();
	const mine = page
		.locator('section')
		.filter({ has: page.getByRole('heading', { name: 'In your collections', exact: true }) });
	const others = page.locator('[data-other-collections]');
	const ownChip = mine.getByRole('link', { name: 'Interiors', exact: true });
	const add = mine.getByRole('button', { name: 'Save to new collection' });
	await expect(ownChip).toBeVisible();
	await expect(add).toHaveCSS('height', '28px');
	await expect(ownChip.locator('..')).toHaveCSS('height', '28px');
	await ownChip.hover();
	const ownCard = page.locator(`[data-collection-hover-card="${own.uri}"]`);
	await expect(ownCard).toBeVisible();
	expect((await ownCard.boundingBox())!.y + (await ownCard.boundingBox())!.height).toBeLessThan(
		(await ownChip.boundingBox())!.y
	);
	await expect(ownCard.getByRole('button', { name: 'Add to favourites' })).toHaveCount(0);
	await expect(others.getByRole('link', { name: 'Interiors', exact: true })).toHaveCount(0);
	await expect(others.getByRole('button', { name: /Remove from / })).toHaveCount(0);
	const foreignChip = others.getByRole('link', { name: 'Inspiration', exact: true });
	await expect(foreignChip).toHaveCSS('height', '28px');
	await expect(foreignChip).toHaveAttribute(
		'href',
		`/profile/${other.handle}/collection/inspiration`
	);
	await foreignChip.hover();
	const card = page.locator(`[data-collection-hover-card="${foreign.uri}"]`);
	await expect(card).toBeVisible();
	await expect(page.locator('[data-collection-hover-card]')).toHaveCount(1);
	const star = card.getByRole('button', { name: 'Add to favourites' });
	const starBox = (await star.boundingBox())!;
	expect(starBox.width).toBe(starBox.height);
	const titleBox = (await card.getByText('Inspiration', { exact: true }).boundingBox())!;
	const countBox = (await card.getByText('4 saves', { exact: true }).boundingBox())!;
	expect(countBox.y - titleBox.y - titleBox.height).toBeLessThanOrEqual(6);
	expect((await star.boundingBox())!.x).toBeGreaterThan(
		(await card.getByText('Inspiration', { exact: true }).boundingBox())!.x
	);
	await star.click();
	await expect(page).toHaveURL('/organize');
	await expect
		.poll(() => calls.favourites)
		.toEqual([{ subjectUri: foreign.uri, subjectCid: foreign.cid }]);
	await expect(card.getByRole('button', { name: 'Remove from favourites' })).toBeVisible();
	await page.screenshot({ path: testInfo.outputPath('other-collections.png') });
	await page.keyboard.press('Escape');
	await expect(card).toHaveCount(0);
	await foreignChip.hover();
	await card.getByRole('button', { name: 'Remove from favourites' }).click();
	await expect.poll(() => calls.unfavourites).toBe(1);
	await others.getByRole('button', { name: 'Show more' }).click();
	await expect(others.getByRole('link', { name: 'More inspiration' })).toBeVisible();
});

for (const target of [own, foreign]) {
	test(`the ${target.name} hover card opens the same destination as its chip`, async ({ page }) => {
		await mockApi(page);
		await page.goto('/organize');
		await page.locator('[data-uri]').first().click();
		const section =
			target === own
				? page
						.locator('section')
						.filter({
							has: page.getByRole('heading', { name: 'In your collections', exact: true })
						})
				: page.locator('[data-other-collections]');
		const chip = section.getByRole('link', { name: target.name, exact: true });
		const destination = await chip.getAttribute('href');
		await chip.hover();
		const card = page.locator(`[data-collection-hover-card="${target.uri}"]`);
		const link = card.getByRole('link', { name: `Open ${target.name}`, exact: true });
		await expect(link).toHaveAttribute('href', destination!);
		await link.click();
		await expect(page).toHaveURL(destination!);
	});
}

test('switching the selected image ignores a late collection response', async ({ page }) => {
	await mockApi(page);
	let release!: () => void;
	const ready = new Promise<void>((resolve) => {
		release = resolve;
	});
	await page.route(`${API}/xrpc/is.currents.feed.getImageCollections?**`, async (route) => {
		const first = new URL(route.request().url()).searchParams.get('uri') === saves[0].uri;
		if (first) await ready;
		await route.fulfill({ json: { collections: [first ? foreign : more] } });
	});
	try {
		await page.goto('/organize');
		await page.locator(`[data-uri="${saves[0].uri}"]`).click();
		await expect(page.locator('[data-other-collections]')).toContainText('Loading collections');
		await page.locator(`[data-uri="${saves[1].uri}"]`).click();
		const others = page.locator('[data-other-collections]');
		await expect(others.getByRole('link', { name: 'More inspiration' })).toBeVisible();
		const lateResponse = page.waitForResponse((response) => {
			const url = new URL(response.url());
			return (
				url.pathname.includes('getImageCollections') && url.searchParams.get('uri') === saves[0].uri
			);
		});
		release();
		await (await lateResponse).finished();
		await expect(others.getByRole('link', { name: 'Inspiration', exact: true })).toHaveCount(0);
	} finally {
		release();
	}
});
