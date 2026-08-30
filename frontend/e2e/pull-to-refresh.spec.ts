import { expect, test, type Page } from '@playwright/test';

const profile = {
	did: 'did:plc:pull-refresh',
	handle: 'pull-refresh.test',
	displayName: 'Pull Refresh',
	followersCount: 0,
	followsCount: 0
};
const collection = {
	uri: `at://${profile.did}/is.currents.feed.collection/ideas`,
	author: profile,
	name: 'Ideas',
	previews: [],
	saveCount: 1,
	createdAt: '2026-08-30T00:00:00Z'
};

function save(rkey: string, alt: string) {
	return {
		uri: `at://${profile.did}/is.currents.feed.save/${rkey}`,
		author: profile,
		content: {
			$type: 'is.currents.content.defs#imageView',
			blobCid: `blob-${rkey}`,
			imageUrl:
				'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
			width: 1,
			height: 1,
			alt
		},
		createdAt: '2026-08-30T00:00:00Z'
	};
}

async function emulateNativeApp(page: Page) {
	// Capacitor detects Android from this bridge before its client module initializes.
	await page.addInitScript(() => {
		Object.defineProperty(window, 'androidBridge', { value: {}, configurable: true });
	});
}

async function pullToRefresh(page: Page, label: string) {
	const client = await page.context().newCDPSession(page);
	const point = { x: 180, y: 180 };
	await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [point] });
	await client.send('Input.dispatchTouchEvent', {
		type: 'touchMove',
		touchPoints: [{ x: point.x, y: point.y + 60 }]
	});
	await expect(page.getByRole('status', { name: `Release to refresh ${label}` })).toBeVisible();
	await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
}

async function mockBase(page: Page, handler: (url: URL) => unknown | undefined) {
	await page.route(/https?:\/\/[^/]+\/(?:xrpc|api)\//, async (route) => {
		const url = new URL(route.request().url());
		const response = handler(url);
		const json = (value: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(value) });

		if (response !== undefined) return json(response);
		if (url.pathname === '/api/me') return route.fulfill({ status: 401, body: '' });
		if (url.pathname.includes('features/seen')) return json({ seen: [] });
		if (url.pathname.includes('moderation/prefs'))
			return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

test('native pull refreshes the feed', async ({ page }) => {
	await emulateNativeApp(page);
	let feedRequests = 0;
	await mockBase(page, (url) => {
		if (url.pathname.endsWith('getFeed')) {
			feedRequests++;
			return {
				feed: [
					save(
						feedRequests === 1 ? 'before' : 'after',
						feedRequests === 1 ? 'Before refresh' : 'After refresh'
					)
				]
			};
		}
		if (url.pathname.endsWith('getActorCollections')) return { collections: [] };
	});

	await page.goto('/explore/general');
	await expect(page.getByRole('img', { name: 'Before refresh' })).toBeVisible();
	await pullToRefresh(page, 'feed');
	await expect(page.getByRole('img', { name: 'After refresh' })).toBeVisible();
	expect(feedRequests).toBe(2);
});

test('native pull refreshes the active profile tab', async ({ page }) => {
	await emulateNativeApp(page);
	let libraryRequests = 0;
	await mockBase(page, (url) => {
		if (url.pathname.endsWith('getProfile')) return profile;
		if (url.pathname.endsWith('getActorCollections')) return { collections: [] };
		if (url.pathname.endsWith('getLibrarySaves')) {
			libraryRequests++;
			return {
				saves: [
					save(
						libraryRequests === 1 ? 'before' : 'after',
						libraryRequests === 1 ? 'Before refresh' : 'After refresh'
					)
				]
			};
		}
		if (url.pathname.endsWith('getFavouriteCollections')) return { collections: [] };
	});

	await page.goto(`/profile/${profile.handle}`);
	await page.getByRole('tab', { name: 'All', exact: true }).click();
	await expect(page.getByRole('img', { name: 'Before refresh' })).toBeVisible();
	await pullToRefresh(page, 'profile');
	await expect(page.getByRole('img', { name: 'After refresh' })).toBeVisible();
	expect(libraryRequests).toBe(2);
});

test('native pull refreshes a collection', async ({ page }) => {
	await emulateNativeApp(page);
	let saveRequests = 0;
	await mockBase(page, (url) => {
		if (url.pathname.endsWith('getProfile')) return profile;
		if (url.pathname.endsWith('getActorCollections')) return { collections: [] };
		if (url.pathname.endsWith('getCollectionSaves')) {
			saveRequests++;
			return {
				collection,
				saves: [
					save(
						saveRequests === 1 ? 'before' : 'after',
						saveRequests === 1 ? 'Before refresh' : 'After refresh'
					)
				]
			};
		}
	});

	await page.goto(`/profile/${profile.handle}/collection/ideas`);
	await expect(page.getByRole('img', { name: 'Before refresh' })).toBeVisible();
	await pullToRefresh(page, 'collection');
	await expect(page.getByRole('img', { name: 'After refresh' })).toBeVisible();
	expect(saveRequests).toBe(2);
});
