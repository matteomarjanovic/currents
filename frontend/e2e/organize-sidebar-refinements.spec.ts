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
	await page.context().route(`${APPVIEW}/**`, (route) => {
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
		if (url.includes('getProfile')) return json(me);
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
		if (url.includes('getSaves')) {
			return json({
				saves: [imageSave(collectionSave, [{ collectionUri: INTERIORS, saveUri: collectionSave }])]
			});
		}
		return json({ saves: [] });
	});
}

test.describe('Organize image links', () => {
	test.use({ hasTouch: false, isMobile: false });
	for (const method of ['ControlOrMeta', 'middle', 'context', 'options'] as const) {
		test(`${method} opens the image detail in another tab and preserves Organize`, async ({
			page,
			context
		}) => {
			const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
			await mockApi(page, calls);
			await page.setViewportSize({ width: 1280, height: 800 });
			await page.goto('/organize');
			const tile = page.locator('[data-uri]').first();
			await tile.click();
			await expect(page.getByRole('button', { name: 'Remove from Interiors' })).toBeVisible();
			await expect(tile).toHaveAttribute('href', `/profile/${me.handle}/save/interiors`);
			const newTab = context.waitForEvent('page');
			if (method === 'middle') await tile.click({ button: 'middle' });
			else if (method === 'ControlOrMeta') await tile.click({ modifiers: ['ControlOrMeta'] });
			else {
				if (method === 'context') await tile.click({ button: 'right' });
				else await page.getByRole('button', { name: 'Options', exact: true }).first().click();
				await page.getByRole('menuitem', { name: 'Open in new tab', exact: true }).click();
			}
			const opened = await newTab;
			await expect(opened).toHaveURL(`/profile/${me.handle}/save/interiors`);
			await expect(
				opened.getByRole('button', { name: 'Go back', exact: true }).first()
			).toBeVisible();
			await expect(page).toHaveURL('/organize');
			await expect(page.getByRole('button', { name: 'Remove from Interiors' })).toBeVisible();
			await opened.close();
		});
	}
});

test('right sidebar collection segment opens the collection in Organize', async ({ page }) => {
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	await mockApi(page, calls);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');
	await page.locator('[data-uri]').first().click();

	const chip = page.locator('[data-slot="badge"]').filter({
		has: page.getByRole('button', { name: 'Remove from Interiors' })
	});
	await chip.getByRole('link', { name: 'Interiors', exact: true }).click();

	await expect(page).toHaveURL(`/organize?c=${encodeURIComponent(INTERIORS)}`);
	expect(calls.deleted).toEqual([]);
	expect(calls.resave).toEqual([]);
});

test('right sidebar Unsorted segment opens Unsorted in Organize', async ({ page }) => {
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	await mockApi(page, calls);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');
	await page.route(`${APPVIEW}/xrpc/is.currents.feed.getLibrarySaves?**`, (route) =>
		route.fulfill({
			json: {
				saves: [
					imageSave(saveUri('unsorted'), [{ collectionUri: '', saveUri: saveUri('unsorted') }])
				]
			}
		})
	);
	await page.reload();
	await page.locator('[data-uri]').first().click();
	await page.getByRole('link', { name: 'Unsorted', exact: true }).click();

	await expect(page).toHaveURL('/organize?unsorted=1');
	await expect(page.locator(`[data-uri="${saveUri('unsorted')}"]`)).toBeVisible();
	expect(calls.deleted).toEqual([]);
});

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

test.describe('desktop related images', () => {
	test.use({ hasTouch: false, isMobile: false });
	test('use Explore collection and save controls without a page scrollbar gutter', async ({
		page
	}) => {
		const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
		await page.addInitScript(
			(uri) => localStorage.setItem('lastUsedCollectionUri', uri),
			INTERIORS
		);
		await mockApi(page, calls);
		await page.route(`${APPVIEW}/xrpc/is.currents.feed.getRelatedSaves?**`, (route) =>
			route.fulfill({ json: { saves: [imageSave(saveUri('related'), [])] } })
		);
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/organize');
		await page.locator('[data-uri]').first().click();
		await page.getByRole('tab', { name: 'Related' }).click();
		const panel = page.getByRole('tabpanel', { name: 'Related' });
		const relatedImage = panel.locator('img').first();
		await expect(relatedImage).toBeVisible();
		await expect
			.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).scrollbarGutter))
			.toBe('auto');
		await relatedImage.hover();
		const selector = panel.getByRole('button', { name: 'Interiors', exact: true });
		const saveButton = panel.getByRole('button', { name: 'Save', exact: true });
		await expect(selector).toBeVisible();
		await expect(saveButton).toHaveClass(/bg-primary/);
		expect((await selector.boundingBox())!.x).toBeLessThan((await saveButton.boundingBox())!.x);
		await saveButton.click();
		await expect.poll(() => calls.resave.length).toBe(1);
		expect(JSON.parse(calls.resave[0])).toEqual({
			saveUri: saveUri('related'),
			collectionUri: INTERIORS
		});
		await expect(panel.getByRole('button', { name: 'Saved', exact: true })).toHaveClass(
			/bg-secondary/
		);
		await page.mouse.move(700, 20);
		await expect(panel.locator('[class*="bg-black/20"]')).toHaveCSS('opacity', '0');
		await relatedImage.hover();
		await selector.click();
		await expect(page.getByText('Save without a collection')).toBeVisible();
		await page.keyboard.press('Escape');
		await relatedImage.click({ button: 'right' });
		await page.getByRole('menuitem', { name: 'View full screen' }).click();
		await expect(page.getByRole('dialog', { name: 'Image viewer' })).toBeVisible();
		await page.getByRole('button', { name: 'Close image viewer' }).click();
		await page.getByRole('tab', { name: 'Details' }).click();
		await expect
			.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).scrollbarGutter))
			.toBe('auto');
	});

	test('context menu opens a related image in a new tab and keeps Organize open', async ({
		page,
		context
	}) => {
		await mockApi(page, { resave: [], deleted: [], unsortedUrls: [] });
		await page.route(`${APPVIEW}/xrpc/is.currents.feed.getRelatedSaves?**`, (route) =>
			route.fulfill({ json: { saves: [imageSave(saveUri('related'), [])] } })
		);
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/organize');
		await page.locator('[data-uri]').first().click();
		await page.getByRole('tab', { name: 'Related' }).click();
		await page
			.getByRole('tabpanel', { name: 'Related' })
			.locator('img')
			.first()
			.click({ button: 'right' });
		const newTab = context.waitForEvent('page');
		await page.getByRole('menuitem', { name: 'Open in new tab', exact: true }).click();
		const opened = await newTab;
		await expect(opened).toHaveURL(`/profile/${me.handle}/save/related`);
		expect(await opened.evaluate(() => window.opener === null)).toBe(true);
		await expect(page).toHaveURL('/organize');
		await expect(page.getByRole('tabpanel', { name: 'Related' })).toBeVisible();
		await opened.close();
	});

	test('right-clicking a second related image keeps the desktop context menu', async ({ page }) => {
		const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
		await mockApi(page, calls);
		await page.route(`${APPVIEW}/xrpc/is.currents.feed.getRelatedSaves?**`, (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					saves: [imageSave(saveUri('related-1'), []), imageSave(saveUri('related-2'), [])]
				})
			})
		);
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/organize');
		await page.locator('[data-uri]').first().click();
		await page.getByRole('tab', { name: 'Related' }).click();

		const images = page.locator('[data-slot="tabs-content"][data-state="active"] img');
		await expect(images).toHaveCount(2);
		await images.first().click({ button: 'right' });
		await expect(page.getByRole('menuitem', { name: 'View full screen' }).last()).toBeVisible();
		// The open menu's dismissal layer can consume pointerdown before a contextmenu
		// reaches the next card. Reproduce that ordering directly.
		await images.nth(1).dispatchEvent('contextmenu', { button: 2, bubbles: true });
		await expect(page.getByText('Quick actions', { exact: true })).not.toBeVisible();
		await expect(page.getByRole('menuitem', { name: 'View full screen' }).last()).toBeVisible();
	});
});

test('related images open the viewer on tap and Quick actions on long press', async ({ page }) => {
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	await mockApi(page, calls);
	await page.route(`${APPVIEW}/xrpc/is.currents.feed.getRelatedSaves?**`, (route) =>
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ saves: [imageSave(saveUri('related'), [])] })
		})
	);
	await page.goto('/organize');
	await page.locator('[data-uri]').first().tap();
	await page.getByRole('tab', { name: 'Related' }).tap();

	const relatedImage = page.getByRole('button', { name: 'View image full screen' });
	await relatedImage.tap();
	await expect(page.getByRole('dialog', { name: 'Image viewer' })).toBeVisible();
	await page.getByRole('button', { name: 'Close image viewer' }).tap();
	await relatedImage.dispatchEvent('pointerdown', { pointerType: 'touch', pointerId: 1 });
	await expect(page.getByText('Quick actions', { exact: true })).toBeVisible();
	await expect(page.locator('[data-slot="context-menu-content"]')).toHaveCount(0);
	await relatedImage.dispatchEvent('pointerup', { pointerType: 'touch', pointerId: 1 });
});

test('mobile related images never open the desktop context menu', async ({ page }) => {
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	await mockApi(page, calls);
	await page.route(`${APPVIEW}/xrpc/is.currents.feed.getRelatedSaves?**`, (route) =>
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ saves: [imageSave(saveUri('related'), [])] })
		})
	);
	await page.goto('/organize');
	await page.locator('[data-uri]').first().tap();
	await page.getByRole('tab', { name: 'Related' }).tap();
	await page.getByRole('button', { name: 'View image full screen' }).click({ button: 'right' });
	await expect(page.locator('[data-slot="context-menu-content"]')).toHaveCount(0);
});

test('Capacitor related images open the drawer without a context menu', async ({ page }) => {
	await page.addInitScript(() => {
		Object.assign(window, {
			androidBridge: {},
			Capacitor: {
				PluginHeaders: [
					{
						name: 'SecurePreferences',
						methods: [{ name: 'get', rtype: 'promise' }]
					}
				],
				async nativePromise(plugin: string, method: string) {
					if (plugin === 'SecurePreferences' && method === 'get') return { value: 'test-token' };
					return {};
				}
			}
		});
	});
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	await mockApi(page, calls);
	await page.route(`${APPVIEW}/xrpc/is.currents.feed.getRelatedSaves?**`, (route) =>
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ saves: [imageSave(saveUri('related'), [])] })
		})
	);
	await page.goto('/organize');
	await page.locator('[data-uri]').first().tap();
	await page.getByRole('tab', { name: 'Related' }).tap();
	const image = page.getByRole('button', { name: 'View image full screen' });
	await image.dispatchEvent('pointerdown', { pointerType: 'touch', pointerId: 1 });
	await expect(page.getByText('Quick actions', { exact: true })).toBeVisible();
	await expect(page.locator('[data-slot="context-menu-content"]')).toHaveCount(0);
	await image.dispatchEvent('pointerup', { pointerType: 'touch', pointerId: 1 });
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

test('desktop sidebar opens on demand and closes after choosing a collection when enabled', async ({
	page
}) => {
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	let autoClose = false;
	await mockApi(page, calls);
	await page.route(`${APPVIEW}/api/preferences`, (route) => {
		if (route.request().method() === 'PUT') {
			autoClose = route.request().postDataJSON().organizeSidebarAutoClose;
			return route.fulfill({ status: 204 });
		}
		return route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ organizeSidebarAutoClose: autoClose })
		});
	});
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/organize');
	const sidebar = page.locator('[data-slot="sidebar"][data-side="left"]');
	await expect(sidebar).toHaveAttribute('data-state', 'expanded');

	await page.goto('/settings/organize');
	await page.getByRole('switch', { name: 'Keep sidebar closed' }).click();
	await expect.poll(() => autoClose).toBe(true);
	await page.goto('/organize');
	await expect(sidebar).toHaveAttribute('data-state', 'collapsed');
	await page.getByRole('button', { name: 'Toggle Sidebar' }).click();
	await expect(sidebar).toHaveAttribute('data-state', 'expanded');
	await page.getByRole('link', { name: 'Interiors' }).click();
	await expect(sidebar).toHaveAttribute('data-state', 'collapsed');
	await page.reload();
	await expect(sidebar).toHaveAttribute('data-state', 'collapsed');

	await page.goto('/settings/organize');
	await page.getByRole('switch', { name: 'Keep sidebar closed' }).click();
	await expect.poll(() => autoClose).toBe(false);
	await page.goto('/organize');
	await expect(sidebar).toHaveAttribute('data-state', 'expanded');
	await page.getByRole('link', { name: 'Interiors' }).click();
	await expect(sidebar).toHaveAttribute('data-state', 'expanded');
});

test('clicking the selected Explore mode opens the Explore feed', async ({ page }) => {
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	await mockApi(page, calls);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/search/saves/cats');
	await page.getByRole('tab', { name: 'Explore' }).click();
	await expect(page).toHaveURL(/\/explore\//);
});

test('clicking Explore in the mobile mode menu opens the Explore feed', async ({ page }) => {
	const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
	await mockApi(page, calls);
	await page.goto('/search/saves/cats');
	await page.getByRole('button', { name: 'Switch mode' }).click();
	await page.getByRole('menuitem', { name: /Explore/ }).click();
	await expect(page).toHaveURL(/\/explore\//);
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

for (const width of [1280, 393]) {
	test(`details keep the canvas thumbnail until the larger image loads at width ${width}`, async ({
		page
	}) => {
		await mockApi(page, { resave: [], deleted: [], unsortedUrls: [] });
		const saves = ['first', 'second'].map((name) => {
			const save = imageSave(saveUri(name), [{ collectionUri: INTERIORS, saveUri: saveUri(name) }]);
			return {
				...save,
				content: {
					...save.content,
					imageUrl: `https://cdn.currents.is/img/${me.did}/${name}`,
					width: 1200,
					height: 1500,
					palette: ['#a7b7ad', '#e4d5c0', '#6a786d']
				}
			};
		});
		await page.route(`${APPVIEW}/xrpc/is.currents.feed.getLibrarySaves?**`, (route) =>
			route.fulfill({ json: { saves } })
		);
		let releaseDetail!: () => void;
		const detailReady = new Promise<void>((resolve) => {
			releaseDetail = resolve;
		});
		await page.route('https://cdn.currents.is/img/**', async (route) => {
			if (new URL(route.request().url()).searchParams.get('quality') === '85') await detailReady;
			await route.fulfill({
				contentType: 'image/svg+xml',
				body: '<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="1500"><rect width="1200" height="1500" fill="#a7b7ad"/><circle cx="600" cy="600" r="300" fill="#e4d5c0"/></svg>'
			});
		});
		await page.setViewportSize({ width, height: 850 });
		await page.goto('/organize');
		const tiles = page.locator('[data-uri]');
		await expect
			.poll(() =>
				tiles
					.first()
					.locator('img')
					.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)
			)
			.toBe(true);
		const thumbnail = await tiles
			.first()
			.locator('img')
			.evaluate((img: HTMLImageElement) => img.currentSrc);
		await tiles.first().click();
		const hero = page.getByRole('button', { name: 'View image full screen', exact: true });
		const preview = hero.locator('[data-detail-preview]');
		await expect(preview).toBeVisible();
		await expect(preview).toHaveCSS('background-image', `url("${thumbnail}")`);
		await expect(hero.locator('img')).toHaveCSS('opacity', '0');
		await expect(hero.locator('[data-slot="skeleton"]')).toHaveCount(0);
		const before = await hero.boundingBox();
		if (width === 1280) {
			// Switching selection while the first image is pending must show the second thumbnail.
			await expect
				.poll(() =>
					tiles
						.nth(1)
						.locator('img')
						.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)
				)
				.toBe(true);
			const secondThumbnail = await tiles
				.nth(1)
				.locator('img')
				.evaluate((img: HTMLImageElement) => img.currentSrc);
			await tiles.nth(1).click();
			await expect(preview).toHaveCSS('background-image', `url("${secondThumbnail}")`);
		}
		releaseDetail();
		await expect(hero.locator('img')).toHaveCSS('opacity', '1');
		await expect(preview).toHaveCount(0);
		expect((await hero.boundingBox())!.height).toBeCloseTo(before!.height, 0);
	});
}

for (const width of [1280, 393]) {
	for (const kind of ['original', 'resave', 'other-author'] as const) {
		test(`Moderation offers the appropriate action for ${kind} at width ${width}`, async ({
			page
		}) => {
			await mockApi(page, { resave: [], deleted: [], unsortedUrls: [] });
			const uri = saveUri('moderation');
			const save = {
				...imageSave(uri, [{ collectionUri: INTERIORS, saveUri: uri }]),
				author: kind === 'other-author' ? { did: 'did:plc:other', handle: 'other.test' } : me,
				...(kind === 'resave'
					? {
							resaveOf: {
								uri: 'at://did:plc:other/is.currents.feed.save/source',
								cid: 'record-cid'
							},
							labels: [
								{
									src: 'did:web:labeler.test',
									val: 'currents-ai-generated',
									cts: '2026-01-01T00:00:00Z'
								}
							]
						}
					: {})
			};
			await page.route(`${APPVIEW}/xrpc/is.currents.feed.getLibrarySaves?**`, (route) =>
				route.fulfill({ json: { saves: [save] } })
			);
			await page.setViewportSize({ width, height: 850 });
			await page.goto('/organize');
			await page.locator('[data-uri]').first().click();
			const moderation = page.locator('section').filter({
				has: page.getByRole('heading', { name: 'Moderation', exact: true })
			});
			if (kind === 'original') {
				await expect(moderation.getByRole('button', { name: 'Report image' })).toHaveCount(0);
				await moderation.getByRole('button', { name: 'Add labels' }).click();
				const dialog = page.getByRole('dialog', { name: 'Content labels' });
				await dialog.getByRole('button', { name: 'AI-generated', exact: true }).click();
				const request = page.waitForRequest(
					(request) =>
						request.url().endsWith('/api/save/moderation/labels') && request.method() === 'PUT'
				);
				await dialog.getByRole('button', { name: 'Apply labels' }).click();
				expect((await request).postData()).toBe('labels=currents-ai-generated');
				await expect(dialog).toBeHidden();
				await expect(moderation.getByText('currents-ai-generated', { exact: true })).toBeVisible();
			} else {
				await expect(moderation.getByRole('button', { name: 'Add labels' })).toHaveCount(0);
				if (kind === 'resave')
					await expect(
						moderation.getByText('currents-ai-generated', { exact: true })
					).toBeVisible();
				await moderation.getByRole('button', { name: 'Report image' }).click();
				const dialog = page.getByRole('dialog', { name: 'Report this image' });
				await dialog.getByRole('radio', { name: 'Violent or graphic content' }).check();
				await dialog.getByLabel('Additional context (optional)').fill('Please review this image.');
				const request = page.waitForRequest(
					(request) =>
						request.url().endsWith('/xrpc/com.atproto.moderation.createReport') &&
						request.method() === 'POST'
				);
				await dialog.getByRole('button', { name: 'Submit report' }).click();
				expect((await request).postDataJSON()).toEqual({
					reasonType: 'violence',
					reason: 'Please review this image.',
					subject: { $type: 'com.atproto.repo.strongRef', uri, cid: '' }
				});
				await expect(dialog).toBeHidden();
			}
		});
	}
}

test('right sidebar resizes without scaling its contents and remembers its width while organizing', async ({
	page
}) => {
	await mockApi(page, { resave: [], deleted: [], unsortedUrls: [] });
	await page.route('https://example.com/**', (route) =>
		route.fulfill({
			contentType: 'image/svg+xml',
			body: '<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="800"><rect width="1200" height="800" fill="#a7b7ad"/></svg>'
		})
	);
	const save = imageSave(saveUri('wide'), [{ collectionUri: INTERIORS, saveUri: saveUri('wide') }]);
	await page.route(`${APPVIEW}/xrpc/is.currents.feed.getLibrarySaves?**`, (route) =>
		route.fulfill({
			json: { saves: [{ ...save, content: { ...save.content, width: 1200, height: 800 } }] }
		})
	);
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto('/organize');
	await page.locator('[data-uri]').first().click();
	const handle = page.getByRole('separator', { name: 'Resize details sidebar' });
	await expect(page.getByRole('button', { name: 'Close details' })).toBeInViewport({ ratio: 1 });
	await expect(handle).toHaveAttribute('aria-valuenow', '352');
	const panel = page.locator(`[id="${await handle.getAttribute('aria-controls')}"]`);
	const canvas = page.locator('[data-slot="sidebar-inset"]');
	const card = panel
		.locator('section')
		.filter({ has: page.getByRole('heading', { name: 'In your collections', exact: true }) });
	const heading = card.getByRole('heading');
	const heroImage = panel.getByRole('button', { name: 'View image full screen' }).locator('img');
	await expect
		.poll(() =>
			heroImage.evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0)
		)
		.toBe(true);
	const imageBefore = (await heroImage.boundingBox())!;
	const action = panel.getByRole('button', { name: 'Find similar in library' });
	const actionBefore = (await action.boundingBox())!;
	const fontBefore = await heading.evaluate((element) => getComputedStyle(element).fontSize);
	const canvasBefore = (await canvas.boundingBox())!;
	const cardBefore = (await card.boundingBox())!;
	const grip = (await handle.boundingBox())!;
	const visibleGrip = (await handle.locator('span').boundingBox())!;
	expect(visibleGrip.x - canvasBefore.x - canvasBefore.width).toBeCloseTo(
		cardBefore.x - visibleGrip.x - visibleGrip.width,
		1
	);
	await expect
		.poll(() => handle.evaluate((element) => getComputedStyle(element, '::before').content))
		.toBe('none');
	expect(grip.x + grip.width).toBeLessThan(cardBefore.x);
	await page.mouse.move(grip.x + grip.width / 2, grip.y + grip.height / 2);
	await page.mouse.down();
	await page.mouse.move(grip.x + grip.width / 2 - 400, grip.y + grip.height / 2, { steps: 10 });
	await page.mouse.up();
	await expect(handle).toHaveAttribute('aria-valuenow', '752');
	await expect(panel).toHaveCSS('width', '752px');
	expect((await canvas.boundingBox())!.width).toBeCloseTo(canvasBefore.width - 400, 0);
	expect((await card.boundingBox())!.width).toBeCloseTo(cardBefore.width + 400, 0);
	expect((await heroImage.boundingBox())!.width).toBeCloseTo(imageBefore.width, 0);
	expect((await heroImage.boundingBox())!.height).toBeCloseTo(imageBefore.height, 0);
	expect((await action.boundingBox())!.width).toBeCloseTo(actionBefore.width, 0);
	await expect(heading).toHaveCSS('font-size', fontBefore);

	await panel.getByRole('button', { name: 'Close details' }).click();
	await expect(handle).toHaveCount(0);
	await page.locator('[data-uri]').first().click();
	await expect(handle).toHaveAttribute('aria-valuenow', '752');
});

test('right sidebar supports keyboard resizing and keeps room for the canvas as the window narrows', async ({
	page
}) => {
	await mockApi(page, { resave: [], deleted: [], unsortedUrls: [] });
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto('/organize');
	await page.locator('[data-uri]').first().click();
	const handle = page.getByRole('separator', { name: 'Resize details sidebar' });
	await expect(handle).toBeInViewport({ ratio: 1 });
	await handle.focus();
	await page.keyboard.press('ArrowLeft');
	await expect(handle).toHaveAttribute('aria-valuenow', '368');
	await page.keyboard.press('ArrowRight');
	await expect(handle).toHaveAttribute('aria-valuenow', '352');
	await page.keyboard.press('ArrowRight');
	await expect(handle).toHaveAttribute('aria-valuenow', '352');
	await expect
		.poll(async () => Number(await handle.getAttribute('aria-valuemax')))
		.toBeGreaterThan(640);
	const maximum = (await handle.getAttribute('aria-valuemax'))!;
	await page.keyboard.press('End');
	await expect(handle).toHaveAttribute('aria-valuenow', maximum);
	await page.setViewportSize({ width: 1100, height: 900 });
	await expect
		.poll(async () => Number(await handle.getAttribute('aria-valuenow')))
		.toBeLessThan(Number(maximum));
	expect(
		(await page.locator('[data-slot="sidebar-inset"]').boundingBox())!.width
	).toBeGreaterThanOrEqual(319);
	await page.keyboard.press('Home');
	await expect(handle).toHaveAttribute('aria-valuenow', '352');
	await page.setViewportSize({ width: 393, height: 850 });
	await expect(handle).toHaveCount(0);
	const panel = page
		.getByRole('button', { name: 'Close details' })
		.locator('xpath=ancestor::div[contains(@class,"fixed")][1]');
	await expect(panel).toHaveCSS('width', '393px');
});

test('Related uses one column in narrow desktop sidebars and two on mobile', async ({ page }) => {
	await mockApi(page, { resave: [], deleted: [], unsortedUrls: [] });
	await page.route(`${APPVIEW}/xrpc/is.currents.feed.getRelatedSaves?**`, (route) =>
		route.fulfill({
			json: { saves: [1, 2, 3, 4].map((n) => imageSave(saveUri(`related-${n}`), [])) }
		})
	);
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto('/organize');
	await page.locator('[data-uri]').first().click();
	await expect(page.getByRole('button', { name: 'Close details' })).toBeInViewport({ ratio: 1 });
	await page.getByRole('tab', { name: 'Related' }).click();
	const frames = page.getByRole('tabpanel', { name: 'Related' }).locator('[data-save-uri]');
	await expect(frames).toHaveCount(4);
	const columnCount = () =>
		frames.evaluateAll(
			(elements) =>
				new Set(elements.map((element) => Math.round(element.getBoundingClientRect().x))).size
		);
	await expect.poll(columnCount).toBe(1);
	await expect.poll(async () => (await frames.first().boundingBox())!.width).toBeGreaterThan(300);

	const handle = page.getByRole('separator', { name: 'Resize details sidebar' });
	const grip = (await handle.locator('span').boundingBox())!;
	const canvas = (await page.locator('[data-slot="sidebar-inset"]').boundingBox())!;
	expect(grip.x - canvas.x - canvas.width).toBeCloseTo(
		(await frames.first().boundingBox())!.x - grip.x - grip.width,
		1
	);
	await handle.focus();
	await page.keyboard.press('End');
	await expect(handle).toHaveAttribute(
		'aria-valuenow',
		(await handle.getAttribute('aria-valuemax'))!
	);
	await expect.poll(columnCount).toBeGreaterThanOrEqual(2);
	await page.keyboard.press('Home');
	await expect.poll(columnCount).toBe(1);
	await page.setViewportSize({ width: 393, height: 850 });
	await expect(handle).toHaveCount(0);
	await expect.poll(columnCount).toBe(2);
});
