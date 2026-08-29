import { test, expect, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';

const overview = {
	now: new Date().toISOString(),
	appview: {
		heapBytes: 1,
		systemBytes: 1,
		goroutines: 1,
		pool: { acquiredConns: 0, idleConns: 1, totalConns: 1, maxConns: 12, emptyAcquireCount: 0 }
	},
	database: {
		sizeBytes: 1,
		connectionCount: 1,
		maxConnections: 100,
		pendingReview: 0,
		largestTables: []
	},
	inference: {
		available: true,
		health: {
			device: 'cpu',
			model: 'test',
			umap: true,
			queues: { text: { pending: 0, max: 8 }, image: { pending: 0, max: 8 } }
		}
	},
	background: {
		missingVisualIdentityCount: 0,
		distinctMissingBlobCidCount: 0,
		collectionsMissingEmbeddingCount: 0
	},
	jobs: [],
	hosts: []
};

async function mockApi(page: Page, role: { admin: boolean; moderatorRole: string | null }) {
	await page.route(`${APPVIEW}/**`, (route) => {
		const url = route.request().url();
		const json = (body: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
		if (url.includes('/api/me/role')) return json(role);
		if (url.includes('/api/admin/overview')) return json(overview);
		if (url.includes('/api/moderation/queue')) return json({ items: [] });
		return json({});
	});
}

test('site admins get the operations dashboard at /admin', async ({ page }) => {
	await mockApi(page, { admin: true, moderatorRole: null });
	await page.goto('/admin');

	await expect(page.getByRole('heading', { name: 'Operations' })).toBeVisible();
	await expect(page.getByText('PostgreSQL', { exact: true })).toBeVisible();
});

test('moderators get the review queue at /moderation', async ({ page }) => {
	await mockApi(page, { admin: false, moderatorRole: 'reviewer' });
	await page.goto('/moderation/queue');

	await expect(page.getByRole('heading', { name: 'Review queue' })).toBeVisible();
	await expect(page.getByText('Currents · moderation')).toBeVisible();
});
