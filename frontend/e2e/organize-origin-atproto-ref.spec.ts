import { test, expect, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const saveUri = `at://${me.did}/is.currents.feed.save/one`;
const origin = {
	uri: 'at://did:plc:source/app.bsky.feed.post/one',
	cid: 'bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku'
};

async function mockApi(page: Page, own = true, initial: typeof origin | null = null) {
	await page.route('https://example.com/**', (route) =>
		route.fulfill({
			status: 200,
			contentType: 'image/svg+xml',
			body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"><rect width="400" height="500" fill="#a7b7ad"/></svg>'
		})
	);
	const writes: { originAtprotoRef: typeof origin | null }[] = [];
	const save = {
		uri: saveUri,
		author: own ? me : { did: 'did:plc:other', handle: 'other.test' },
		createdAt: '2026-02-01T00:00:00Z',
		// Resaves may carry their own editable provenance.
		resaveOf: { uri: 'at://did:plc:other/is.currents.feed.save/source', cid: origin.cid },
		originAtprotoRef: initial ?? undefined,
		content: {
			$type: 'is.currents.content.defs#imageView',
			imageUrl: 'https://example.com/image.jpg',
			blobCid: 'cid-image',
			width: 400,
			height: 500
		},
		viewer: { saves: own ? [{ collectionUri: '', saveUri }] : [] }
	};
	await page.route(`${APPVIEW}/**`, (route) => {
		const request = route.request();
		const url = request.url();
		const json = (body: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
		if (url.endsWith('/api/save/one/origin-atproto-ref') && request.method() === 'PUT') {
			const body = request.postDataJSON();
			writes.push(body);
			save.originAtprotoRef = body.originAtprotoRef ?? undefined;
			return json({ uri: saveUri, ...body });
		}
		if (url.includes('/api/me/role')) return json({ role: 'user' });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('/api/supporter/status')) return json({ active: true, subscribed: true });
		if (url.includes('getActorCollections') || url.includes('getFavouriteCollections'))
			return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		if (url.includes('/api/preferences')) return json({});
		if (url.includes('getFeed')) return json({ feed: [save], cursor: null });
		if (url.includes('getLibrarySaves')) return json({ saves: [save] });
		return json({ saves: [] });
	});
	return writes;
}

for (const width of [1280, 393]) {
	test(`owner can add, edit, and clear provenance in Organize at width ${width}`, async ({
		page
	}) => {
		const writes = await mockApi(page);
		await page.setViewportSize({ width, height: 850 });
		await page.goto('/organize');
		await page.locator('[data-uri]').first().click();
		await page.getByRole('button', { name: 'Add AT Protocol source' }).click();
		const dialog = page.getByRole('dialog', { name: 'AT Protocol source' });
		await dialog.getByLabel('Record URI').fill(origin.uri);
		await dialog.getByRole('button', { name: 'Save', exact: true }).click();
		await expect(dialog.getByRole('alert')).toContainText('Enter both');
		expect(writes).toHaveLength(0);
		await dialog.getByLabel('Record CID').fill(origin.cid);
		await dialog.getByRole('button', { name: 'Save', exact: true }).click();
		await expect(dialog).toBeHidden();
		expect(writes[0]).toEqual({ originAtprotoRef: origin });
		await expect(page.getByText(origin.uri, { exact: true })).toBeVisible();

		await page.getByRole('button', { name: 'Edit AT Protocol source' }).click();
		await expect(dialog.getByLabel('Record URI')).toHaveValue(origin.uri);
		await dialog.getByLabel('Record URI').fill(origin.uri + 'edited');
		await dialog.getByRole('button', { name: 'Save', exact: true }).click();
		await expect(dialog).toBeHidden();
		expect(writes[1]).toEqual({ originAtprotoRef: { ...origin, uri: origin.uri + 'edited' } });

		await page.getByRole('button', { name: 'Edit AT Protocol source' }).click();
		await dialog.getByLabel('Record URI').fill('');
		await dialog.getByLabel('Record CID').fill('');
		await dialog.getByRole('button', { name: 'Save', exact: true }).click();
		await expect(dialog).toBeHidden();
		expect(writes[2]).toEqual({ originAtprotoRef: null });
		await expect(page.getByRole('button', { name: 'Add AT Protocol source' })).toBeVisible();
		await page.reload();
		await page.locator('[data-uri]').first().click();
		await expect(page.getByRole('button', { name: 'Add AT Protocol source' })).toBeVisible();
	});
}

test('another author’s provenance is visible but read-only in Organize', async ({ page }) => {
	const writes = await mockApi(page, false, origin);
	await page.setViewportSize({ width: 1280, height: 850 });
	await page.goto('/organize');
	await page.locator('[data-uri]').first().click();
	await expect(page.getByText(origin.uri, { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Edit AT Protocol source' })).toBeHidden();
	await expect(page.getByRole('button', { name: 'Add AT Protocol source' })).toBeHidden();
	expect(writes).toHaveLength(0);
});

test('Explore details do not show the provenance field', async ({ page }) => {
	await mockApi(page, true, origin);
	await page.goto('/explore/general');
	await page.locator('a.block:has(img)').first().click();
	await expect(page.getByRole('button', { name: 'View image full screen' })).toBeVisible();
	await expect(page.getByText('AT Protocol source', { exact: true })).toBeHidden();
	await expect(page.getByText(origin.uri, { exact: true })).toBeHidden();
});
