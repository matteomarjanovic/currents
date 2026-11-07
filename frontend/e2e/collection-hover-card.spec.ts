import { test, expect, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social' };
const uri = (key: string) => `at://${me.did}/is.currents.collection/${key}`;
const collection = {
	uri: uri('interiors'),
	name: 'Interiors',
	description: 'Rooms and spaces worth remembering.',
	saveCount: 12,
	sectionCount: 1,
	previews: [1, 2, 3, 4].map((i) => ({ url: `${APPVIEW}/img/${me.did}/preview${i}` }))
};
const section = { uri: uri('kitchens'), parentUri: collection.uri, name: 'Kitchens', saveCount: 3 };
const empty = { uri: uri('empty'), name: 'Empty collection', saveCount: 0 };
const favourite = {
	...collection,
	uri: uri('favourite'),
	name: 'Favourite places',
	author: { did: 'did:plc:other', handle: 'other.bsky.social' }
};

async function mockApi(page: Page) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		if (url.includes('/img/'))
			return route.fulfill({
				contentType: 'image/svg+xml',
				body: '<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200"><rect width="200" height="200" fill="#6a9a85"/></svg>'
			});
		const json = (value: unknown) => route.fulfill({ json: value });
		if (url.includes('/api/me/role')) return json({ role: 'user' });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('getActorCollections'))
			return json({ collections: [collection, section, empty] });
		if (url.includes('getFavouriteCollections')) return json({ collections: [favourite] });
		if (url.includes('/api/preferences')) return json({ organizeCollectionSort: 'name' });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		if (url.includes('features/seen')) return json({ seen: [] });
		return json({ saves: [] });
	});
}

test.describe('desktop collection previews', () => {
	test.use({ viewport: { width: 1280, height: 900 }, hasTouch: false, isMobile: false });
	test('hover opens to the right, stays open over the card, and preserves navigation and actions', async ({
		page
	}, testInfo) => {
		await mockApi(page);
		await page.goto('/organize');
		const link = page.getByRole('link', { name: 'Interiors', exact: true });
		await expect(link).toBeVisible();
		await expect(page.locator('[data-collection-hover-card]')).toHaveCount(0);
		await link.hover();
		const card = page.locator(`[data-collection-hover-card="${collection.uri}"]`);
		await expect(card).toBeVisible();
		await expect(card.getByText(collection.description)).toHaveCount(0);
		await expect(card.locator('..')).toHaveCSS('padding-top', '4px');
		await expect(card).toContainText('12 saves');
		await expect(card).toContainText('1 section');
		await expect(card.locator('img')).toHaveCount(4);
		const linkBox = (await link.boundingBox())!;
		const cardBox = (await card.boundingBox())!;
		expect(cardBox.x).toBeGreaterThan(linkBox.x + linkBox.width);
		await card.hover();
		await expect(card).toBeVisible();
		await page.screenshot({ path: testInfo.outputPath('collection-hover-card.png') });
		await page.keyboard.press('Escape');
		await expect(card).toHaveCount(0);
		await link.click({ button: 'right' });
		await expect(page.getByRole('menuitem', { name: 'Edit', exact: true })).toBeVisible();
		await expect(card).toHaveCount(0);
		await page.keyboard.press('Escape');
		await link.click();
		await expect(page).toHaveURL(`/organize?c=${encodeURIComponent(collection.uri)}`);
	});

	test('moving near the next row replaces the previous preview', async ({ page }) => {
		await mockApi(page);
		await page.goto('/organize');
		const first = page.getByRole('link', { name: 'Empty collection', exact: true });
		const second = page.getByRole('link', { name: 'Interiors', exact: true });
		const firstBox = (await first.boundingBox())!;
		const secondBox = (await second.boundingBox())!;
		await page.mouse.move(firstBox.x + firstBox.width - 35, firstBox.y + firstBox.height / 2);
		await expect(page.locator(`[data-collection-hover-card="${empty.uri}"]`)).toBeVisible();
		await page.mouse.move(secondBox.x + secondBox.width - 30, secondBox.y + 2, { steps: 10 });
		await expect(page.locator(`[data-collection-hover-card="${collection.uri}"]`)).toBeVisible();
		await expect(page.locator('[data-collection-hover-card]')).toHaveCount(1);
		await page.mouse.move(firstBox.x + firstBox.width - 35, firstBox.y + firstBox.height - 2, {
			steps: 10
		});
		await expect(page.locator(`[data-collection-hover-card="${empty.uri}"]`)).toBeVisible();
		await expect(page.locator('[data-collection-hover-card]')).toHaveCount(1);
	});

	test('sections, empty collections, and favourites have previews', async ({ page }) => {
		await mockApi(page);
		await page.goto(`/organize?c=${encodeURIComponent(section.uri)}`);
		await page.locator('[data-sidebar="menu-sub-button"]').filter({ hasText: 'Kitchens' }).hover();
		await expect(page.locator(`[data-collection-hover-card="${section.uri}"]`)).toContainText(
			'3 saves'
		);
		await page.getByRole('link', { name: 'Empty collection', exact: true }).hover();
		const card = page.locator(`[data-collection-hover-card="${empty.uri}"]`);
		await expect(card).toBeVisible();
		await expect(card).toContainText('0 saves');
		await expect(card.locator('img')).toHaveCount(0);
		await page.getByRole('button', { name: 'Favourites from others' }).click();
		await page.getByRole('link', { name: 'Favourite places', exact: true }).hover();
		await expect(page.locator(`[data-collection-hover-card="${favourite.uri}"]`)).toContainText(
			'@other.bsky.social'
		);
	});
});

test('mobile collection links navigate without opening a hover card', async ({ page }) => {
	await mockApi(page);
	await page.goto('/organize');
	await page.getByRole('button', { name: 'Toggle Sidebar' }).click();
	const link = page.getByRole('link', { name: 'Interiors', exact: true });
	await link.hover();
	await expect(page.locator('[data-collection-hover-card]')).toHaveCount(0);
	await link.tap();
	await expect(page).toHaveURL(`/organize?c=${encodeURIComponent(collection.uri)}`);
	await expect(page.locator('[data-collection-hover-card]')).toHaveCount(0);
});

test('Explore collection rows have square thumbnails, modest insets, and hover previews', async ({
	page
}, testInfo) => {
	await page.setViewportSize({ width: 1440, height: 1000 });
	await mockApi(page);
	const save = {
		uri: `at://${me.did}/is.currents.save/s1`,
		author: me,
		createdAt: '2026-01-01T00:00:00Z',
		content: {
			$type: 'is.currents.content.defs#imageView',
			imageUrl: `${APPVIEW}/img/${me.did}/s1`,
			blobCid: 's1',
			width: 400,
			height: 500
		},
		viewer: { saves: [] }
	};
	await page.route(`${APPVIEW}/xrpc/is.currents.feed.getFeed?**`, (route) =>
		route.fulfill({ json: { feed: [save] } })
	);
	await page.route(`${APPVIEW}/xrpc/is.currents.feed.getImageCollections?**`, (route) =>
		route.fulfill({ json: { collections: [favourite, empty] } })
	);
	await page.goto('/explore/general');
	await page.locator('a.block:has(img)').first().click();
	const overlay = page.locator('[data-save-detail-overlay]');
	await overlay
		.getByRole('button', { name: 'In other collections' })
		.filter({ visible: true })
		.click();
	const row = overlay.locator('[data-slot="item"]').filter({ visible: true }).first();
	await expect(row).toBeVisible();
	await expect(row).toHaveCSS('padding-left', '14px');
	const accordionContent = row.locator('xpath=ancestor::*[@data-slot="accordion-content"]');
	await expect(accordionContent).toHaveCSS('padding-left', '12px');
	const thumbnail = row.locator('[data-collection-thumbnail]');
	const rowBox = (await row.boundingBox())!;
	const thumbnailBox = (await thumbnail.boundingBox())!;
	expect(thumbnailBox.width).toBe(thumbnailBox.height);
	const topInset = thumbnailBox.y - rowBox.y;
	const bottomInset = rowBox.y + rowBox.height - thumbnailBox.y - thumbnailBox.height;
	expect(Math.abs(topInset - bottomInset)).toBeLessThanOrEqual(1);
	expect(await row.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(
		await overlay.evaluate((element) => getComputedStyle(element).backgroundColor)
	);
	expect(await row.evaluate((element) => getComputedStyle(element).backgroundImage)).toBe(
		await overlay.evaluate((element) => getComputedStyle(element).backgroundImage)
	);
	await row.hover();
	const preview = page.locator(`[data-collection-hover-card="${favourite.uri}"]`);
	await expect(preview).toBeVisible();
	await expect(page.locator('[data-collection-hover-card]')).toHaveCount(1);
	await expect(preview.locator('..')).toHaveAttribute('data-side', 'right');
	await page.screenshot({ path: testInfo.outputPath('explore-collection-cards.png') });
	await page.keyboard.press('Escape');
	await expect(preview).toHaveCount(0);
	await expect(row.getByRole('link', { name: favourite.name, exact: true })).toHaveAttribute(
		'href',
		`/profile/${favourite.author.handle}/collection/favourite`
	);
	await expect(row.getByRole('button', { name: 'Add to favourites' })).toBeVisible();
	await page.mouse.move(1000, 80);
	await row.hover();
	const cardLink = preview.getByRole('link', { name: `Open ${favourite.name}`, exact: true });
	await expect(cardLink).toHaveAttribute(
		'href',
		`/profile/${favourite.author.handle}/collection/favourite`
	);
	await cardLink.click();
	await expect(page).toHaveURL(`/profile/${favourite.author.handle}/collection/favourite`);
});
