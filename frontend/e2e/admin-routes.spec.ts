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
		missingVisualIdentityCount: 193,
		distinctMissingBlobCidCount: 191,
		collectionsMissingEmbeddingCount: 1,
		pendingEnrichmentCount: 2,
		failedEnrichmentCount: 2,
		unsupportedMediaCount: 144,
		invalidMediaCount: 41,
		unavailableSourceCount: 4,
		oldestMissingAgeSec: 129 * 86400,
		oldestPendingAgeSec: 20 * 60
	},
	jobs: [],
	hosts: [
		{
			host: 'main',
			reportedAt: new Date().toISOString(),
			payload: {
				memory: { totalBytes: 8e9, availableBytes: 4e9 },
				load1: 0.5,
				storage: { path: '/mnt/pgdata', totalBytes: 30e9, usedBytes: 16e9, availableBytes: 12e9 },
				rootStorage: { path: '/', totalBytes: 60e9, usedBytes: 24e9, availableBytes: 33e9 },
				containers: [],
				modelVersion: null,
				modelUpdatedAt: null
			}
		}
	]
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
	await expect(page.getByText('24 GB / 60 GB (33 GB free)', { exact: true })).toHaveCount(2);
	await expect(page.getByText('Root disk', { exact: true })).toBeVisible();
	await expect(page.getByText('Operating system and Docker storage.')).toBeVisible();
	const pipeline = page.getByTestId('enrichment-status');
	for (const [label, count] of [
		['Pending', 2],
		['Retry needed', 2],
		['Unsupported media', 144],
		['Invalid images', 41],
		['Source unavailable', 4],
		['Collection embeddings', 1]
	] as const) {
		await expect(
			pipeline.getByText(label, { exact: true }).locator('..').locator('p').first()
		).toHaveText(String(count));
	}
	await expect(pipeline).toContainText('193 unresolved saves · 191 distinct blobs.');
	await expect(pipeline).toContainText('Oldest pending: 20m ago');
	await expect(pipeline).not.toContainText('129d ago');
});

test('moderators get the review queue at /moderation', async ({ page }) => {
	await mockApi(page, { admin: false, moderatorRole: 'reviewer' });
	await page.goto('/moderation/queue');

	await expect(page.getByRole('heading', { name: 'Review queue' })).toBeVisible();
	await expect(page.getByText('Currents · moderation')).toBeVisible();
});
