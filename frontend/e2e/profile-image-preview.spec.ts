import { test, expect, type Page } from '@playwright/test';

// Regression test: picking an avatar/banner in the edit-profile dialog must show a preview.
// It used to vanish instantly because the `if (open) resetForm()` effect read the object-URL
// state (via the revoke helpers), so creating a new object URL re-fired the effect and reset
// the form. resetForm() is now untracked.

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const profile = { ...me, description: '', followersCount: 0, followsCount: 0 };

// A valid 1x1 PNG.
const PNG = Buffer.from(
	'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
	'base64'
);

async function mockApi(page: Page) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (o: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(o) });
		if (url.includes('/api/me')) return json(me);
		if (url.includes('getProfile')) return json(profile);
		if (url.includes('getActorCollections')) return json({ collections: [] });
		if (url.includes('getUnsortedSaves')) return json({ saves: [], cursor: null });
		if (url.includes('getFavouriteCollections')) return json({ collections: [], cursor: null });
		if (url.includes('features/seen')) return json({ seen: [] });
		if (url.includes('moderation/prefs')) return json({ adult: 'blur', aiGenerated: 'show' });
		return json({});
	});
}

test('edit-profile dialog keeps the preview of a newly picked image', async ({ page }) => {
	await mockApi(page);
	await page.goto('/profile/test.bsky.social');

	await page.getByRole('button', { name: /Edit profile/i }).click();
	const dialog = page.getByRole('dialog');
	await expect(dialog).toBeVisible();

	// Pick a banner image via the web file input.
	await dialog.locator('#profile-banner').setInputFiles({
		name: 'banner.png',
		mimeType: 'image/png',
		buffer: PNG
	});

	// The preview must appear AND still be there a moment later (the bug wiped it instantly).
	const preview = dialog.locator('img[src^="blob:"]');
	await expect(preview).toBeVisible({ timeout: 3000 });
	await page.waitForTimeout(300);
	await expect(preview).toBeVisible();
});
