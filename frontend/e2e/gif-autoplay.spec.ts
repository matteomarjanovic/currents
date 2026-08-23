import { test, expect, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const CDN = 'https://cdn.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const gifUrl = `${CDN}/img/${me.did}/bafygif`;
const gifSave = {
	uri: `at://${me.did}/is.currents.feed.save/gif`,
	author: me,
	content: {
		$type: 'is.currents.content.defs#imageView',
		blobCid: 'bafygif',
		imageUrl: gifUrl,
		mimeType: 'image/gif',
		width: 400,
		height: 500,
		alt: 'Animated image'
	},
	createdAt: '2026-01-01T00:00:00Z',
	viewer: { saves: [] }
};

async function mockApi(page: Page) {
	await page.route(`${CDN}/**`, (route) =>
		route.fulfill({
			status: 200,
			contentType: 'image/svg+xml',
			body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"></svg>'
		})
	);
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (value: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(value) });

		if (url.includes('/api/me')) return json(me);
		if (url.includes('/api/preferences')) return json({ gifAutoplay: true });
		if (url.includes('/api/feed/preferences')) {
			return json({ excludedCollections: [], defaultFeed: 'general' });
		}
		if (url.includes('getFeed')) return json({ feed: [gifSave], cursor: null });
		if (url.includes('getActorCollections')) return json({ collections: [] });
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
}

test('autoplay GIFs bypass the image optimizer that strips animation frames', async ({ page }) => {
	await mockApi(page);
	await page.goto('/explore/general');

	const image = page.getByAltText('Animated image');
	await expect(image).toBeVisible();
	await expect(image).toHaveAttribute('src', gifUrl);
	await expect(image).not.toHaveAttribute('srcset', /.+/);
});
