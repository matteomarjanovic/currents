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

test.describe('desktop related images', () => {
	test.use({ hasTouch: false, isMobile: false });
	test('show Save and image actions without a page scrollbar gutter', async ({ page }) => {
		const calls: Calls = { resave: [], deleted: [], unsortedUrls: [] };
		await mockApi(page, calls);
		await page.route(`${APPVIEW}/xrpc/is.currents.feed.getRelatedSaves?**`, (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					saves: [
						imageSave(saveUri('related'), [
							{ collectionUri: INTERIORS, saveUri: saveUri('related') }
						])
					]
				})
			})
		);
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/organize');
		await page.locator('[data-uri]').first().click();
		await page.getByRole('tab', { name: 'Related' }).click();

		const relatedImage = page
			.locator('[data-slot="tabs-content"][data-state="active"] img')
			.first();
		await expect(relatedImage).toBeVisible();
		await expect
			.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).scrollbarGutter))
			.toBe('auto');
		await relatedImage.hover();
		const saveButton = page.getByRole('button', { name: 'Save', exact: true });
		const actionsButton = page.getByRole('button', { name: 'Image actions' });
		await expect(saveButton).toBeVisible();
		await expect(actionsButton).toBeVisible();
		await expect(saveButton).toHaveClass(/bg-primary/);
		await expect(actionsButton).toHaveClass(/bg-secondary/);
		expect((await saveButton.boundingBox())!.x).toBeLessThan(
			(await actionsButton.boundingBox())!.x
		);
		await saveButton.click();
		await expect(page.getByText('Save without a collection')).toBeVisible();
		await expect(saveButton).toHaveText('Save');
		await page.keyboard.press('Escape');
		await actionsButton.click();
		await page.getByRole('menuitem', { name: 'View full screen' }).click();
		await expect(page.getByRole('dialog', { name: 'Image viewer' })).toBeVisible();
		await page.getByRole('button', { name: 'Close image viewer' }).click();
		await relatedImage.click({ button: 'right' });
		await page.getByRole('menuitem', { name: 'View full screen' }).click();
		await expect(page.getByRole('dialog', { name: 'Image viewer' })).toBeVisible();
		await page.getByRole('button', { name: 'Close image viewer' }).click();
		await page.getByRole('tab', { name: 'Details' }).click();
		await expect
			.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).scrollbarGutter))
			.toBe('auto');
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
