import { test, expect, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const profile = {
	did: 'did:plc:profile-pagination',
	handle: 'many-collections.test',
	displayName: 'Many Collections',
	followersCount: 0,
	followsCount: 0
};

function collection(index: number) {
	return {
		uri: `at://${profile.did}/is.currents.feed.collection/c${index}`,
		author: profile,
		name: `Collection ${index}`,
		previews: [],
		saveCount: index,
		createdAt: `2026-01-${String(31 - index).padStart(2, '0')}T00:00:00Z`
	};
}

const allSave = {
	uri: `at://${profile.did}/is.currents.feed.save/all-1`,
	author: profile,
	content: {
		$type: 'is.currents.content.defs#imageView',
		blobCid: 'blob-all-1',
		imageUrl:
			'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
		width: 1,
		height: 1,
		alt: 'All tab image'
	},
	createdAt: '2026-01-31T00:00:00Z'
};

async function mockApi(page: Page, collectionRequests: URL[], allRequests: URL[]) {
	await page.route(`${APPVIEW}/**`, async (route) => {
		const url = new URL(route.request().url());
		const json = (value: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(value) });

		if (url.pathname === '/api/me') return route.fulfill({ status: 401, body: '' });
		if (url.pathname.endsWith('getProfile')) return json(profile);
		if (url.pathname.endsWith('getActorCollections')) {
			collectionRequests.push(url);
			return url.searchParams.get('cursor') === 'next'
				? json({ collections: [collection(17)] })
				: json({
						collections: Array.from({ length: 16 }, (_, i) => collection(i + 1)),
						cursor: 'next'
					});
		}
		if (url.pathname.endsWith('getLibrarySaves')) {
			allRequests.push(url);
			return json({ saves: [allSave] });
		}
		if (url.pathname.includes('features/seen')) return json({ seen: [] });
		if (url.pathname.includes('moderation/prefs'))
			return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

test('profile collections and All images load incrementally', async ({ page }) => {
	const collectionRequests: URL[] = [];
	const allRequests: URL[] = [];
	await mockApi(page, collectionRequests, allRequests);
	await page.goto(`/profile/${profile.handle}`);

	await expect(page.getByText('Collection 16', { exact: true })).toBeVisible();
	expect(collectionRequests).toHaveLength(1);
	expect(collectionRequests[0].searchParams.get('limit')).toBe('16');
	expect(collectionRequests[0].searchParams.get('cursor')).toBeNull();
	await expect(page.getByText('Collection 17', { exact: true })).toHaveCount(0);

	await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
	await expect(page.getByText('Collection 17', { exact: true })).toBeVisible();
	expect(collectionRequests).toHaveLength(2);
	expect(collectionRequests[1].searchParams.get('cursor')).toBe('next');

	await page.getByRole('tab', { name: 'All', exact: true }).click();
	await expect(page.getByRole('img', { name: 'All tab image' })).toBeVisible();
	expect(allRequests).toHaveLength(1);
	expect(allRequests[0].searchParams.get('actor')).toBe(profile.handle);
	expect(allRequests[0].searchParams.get('limit')).toBe('50');
});
