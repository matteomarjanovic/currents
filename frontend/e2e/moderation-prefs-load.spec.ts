import { test, expect, type Page } from '@playwright/test';

// Regression test: the viewer's moderation preferences must be loaded on every route, not
// only the ones that render the top bar. The standalone save page lives in the
// (without-navbar) group, so landing on it directly (a refresh, a shared link) used to leave
// the store at its blur-everything defaults — a viewer who had set adult content to "show"
// saw it blurred anyway, and one who had set it to "hide" saw it blurred instead of gone.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const profile = { ...me, description: '', followersCount: 0, followsCount: 0 };

const showEverything = {
	porn: 'show',
	sexual: 'show',
	nudity: 'show',
	graphicMedia: 'show',
	aiGenerated: 'show'
};

const save = {
	uri: `at://${me.did}/is.currents.feed.save/s1`,
	author: me,
	content: {
		$type: 'is.currents.content.defs#imageView',
		blobCid: 'bafy1',
		imageUrl: `${APPVIEW}/img/${me.did}/bafy1?w=400&h=500`,
		width: 400,
		height: 500
	},
	createdAt: '2026-01-01T00:00:00Z',
	viewer: { saves: [] },
	labels: [{ src: 'did:web:labeler.test', val: 'nudity', cts: '2026-01-01T00:00:00Z' }]
};

// Holds the prefs response open so the pending window can be observed, instead of
// racing a real one that lands in milliseconds.
function prefsGate() {
	let release!: () => void;
	const held = new Promise<void>((r) => (release = r));
	return { held, release };
}

async function mockApi(page: Page, gate?: { held: Promise<void> }) {
	await page.route(`${APPVIEW}/**`, async (route) => {
		const url = route.request().url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/img/')) {
			return route.fulfill({
				status: 200,
				contentType: 'image/svg+xml',
				body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"><rect width="100%" height="100%" fill="#3b82f6"/></svg>'
			});
		}
		if (url.includes('/api/me')) return json(me);
		if (url.includes('moderation/prefs')) {
			if (gate) await gate.held;
			return json(showEverything);
		}
		if (url.includes('getProfile')) return json(profile);
		if (url.includes('getSaves')) return json({ saves: [save] });
		if (url.includes('getRelatedSaves')) return json({ saves: [] });
		if (url.includes('getImageCollections')) return json({ collections: [] });
		if (url.includes('getActorCollections')) return json({ collections: [] });
		if (url.includes('features/seen')) return json({ seen: [] });
		return json({});
	});
}

test('a labeled save opened directly honors the viewer\'s "show" preference', async ({ page }) => {
	await mockApi(page);
	await page.goto('/profile/test.bsky.social/save/s1');

	// The image renders (it isn't filtered out) and no blur overlay survives the prefs load.
	// `.last()`: the desktop hero renders the same markup behind `md:hidden`.
	await expect(page.locator('img[src*="/img/"]').last()).toBeVisible();
	await expect(page.getByText('Nudity')).toBeHidden();
	await expect(page.getByRole('button', { name: 'Show' })).toBeHidden();
});

test('a labeled image is withheld until the prefs land, never guessed at', async ({ page }) => {
	const gate = prefsGate();
	await mockApi(page, gate);
	await page.goto('/profile/test.bsky.social/save/s1');

	// Prefs are still in flight: neither the image nor the default blur overlay may
	// render, or the viewer sees a decision that hasn't been made yet.
	await expect(page.getByRole('button', { name: /^Save$/ }).last()).toBeVisible();
	await expect(page.locator('img[src*="/img/"]')).toHaveCount(0);
	await expect(page.getByText('Nudity')).toHaveCount(0);

	gate.release();

	await expect(page.locator('img[src*="/img/"]').last()).toBeVisible();
	await expect(page.getByText('Nudity')).toBeHidden();
});
