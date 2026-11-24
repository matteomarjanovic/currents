import { test, expect, type Page } from '@playwright/test';

const api = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const subject = `at://${me.did}/is.currents.feed.save/photo`;
const issuer = 'did:plc:ar7c4by46qjdydhdevvrndac';
const timestamp = '2026-11-23T00:00:00Z';
const entry = {
	uri: subject,
	previewUrl: `${api}/img/${me.did}/photo`,
	notice: {
		src: issuer,
		isRead: false,
		updatedAt: timestamp,
		enforcement: { state: 'labeled', labels: ['graphic-media'] },
		latestAction: {
			type: 'labelApplied',
			createdAt: timestamp,
			labels: ['graphic-media'],
			policies: [{ displayName: 'Graphic content policy' }]
		},
		availableActions: ['appeal']
	}
};

async function mockAPI(page: Page, unavailable = false) {
	await page.route(`${api}/**`, async (route) => {
		const url = new URL(route.request().url());
		const json = (data: unknown) => route.fulfill({ json: data });
		switch (url.pathname) {
			case '/api/me':
				return json(me);
			case '/api/me/attestations':
				return json({
					items: [
						{
							id: 7,
							source: 'label_applied',
							subjectUri: subject,
							category: 'nsfw',
							labelVal: 'nudity',
							createdAt: timestamp
						}
					]
				});
			case '/api/me/attestations/7/ignore':
				return route.fulfill({ status: 204 });
			case '/api/me/bluesky-moderation':
				if (unavailable) return route.fulfill({ status: 503 });
				if (url.searchParams.has('subject')) {
					const updated = {
						...entry,
						notice: { ...entry.notice, isRead: true, availableActions: [] }
					};
					return json({ status: 'available', items: [updated] });
				}
				if (url.searchParams.has('cursor')) return json({ status: 'available', items: [entry] });
				return json({ status: 'available', items: [entry], cursor: 'older' });
			case '/api/moderation/prefs':
				return json({ graphicMedia: 'blur', aiGenerated: 'show' });
			case '/xrpc/is.currents.feed.getFeed':
				return json({ feed: [] });
			case '/xrpc/is.currents.graph.getActorCollections':
				return json({ collections: [] });
			default:
				return json({});
		}
	});
}

test('moderation notices retain provenance, deduplicate, and use Bluesky capabilities', async ({
	page
}) => {
	await mockAPI(page);
	await page.goto('/explore/general?bluesky-moderation=connected');

	const notices = page.getByRole('region', { name: 'Bluesky moderation notices' });
	await expect(notices.getByText('Unread in Bluesky')).toBeVisible();
	await expect(notices.getByText('Graphic content policy')).toBeVisible();
	await expect(notices.getByRole('img', { name: 'Currents save' })).toBeVisible();
	const appeal = notices.getByRole('link', { name: 'Appeal in Bluesky' });
	await expect(appeal).toHaveAttribute(
		'href',
		/^https:\/\/bsky.app\/moderation\/inbox\/notice\/details\?/
	);
	await expect(page.getByRole('button', { name: 'Acknowledge' })).toBeVisible();

	await notices.getByRole('button', { name: 'Load older Bluesky notices' }).click();
	await expect(notices.getByRole('button', { name: 'Load older Bluesky notices' })).toBeHidden();
	await expect(notices.getByRole('article')).toHaveCount(1);

	await notices.getByRole('button', { name: 'Check decision' }).click();
	await expect(notices.getByText('Unread in Bluesky')).toBeHidden();
	await expect(appeal).toBeHidden();
});

test('Currents actions remain available during a Bluesky outage', async ({ page }) => {
	await mockAPI(page, true);
	await page.goto('/explore/general?bluesky-moderation=connected');
	await expect(page.getByText('Bluesky moderation is temporarily unavailable.')).toBeVisible();
	const action = page.waitForRequest((r) => r.url().endsWith('/api/me/attestations/7/ignore'));
	await page.getByRole('button', { name: 'Acknowledge' }).click();
	expect((await action).method()).toBe('POST');
});
