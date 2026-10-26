import { test, expect, type Page } from '@playwright/test';
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import type { ImportJob } from '../src/lib/pinterest-import';

const APPVIEW = 'https://api-dev.currents.is';
const previewDir = resolve('test-results/pinterest-preview');
const session = {
	sessionId: '11111111-1111-1111-1111-111111111111',
	username: 'pinterest-user',
	startedAt: '2026-10-22T02:55:00Z'
};
const prior = {
	...session,
	sessionId: '22222222-2222-2222-2222-222222222222',
	startedAt: '2026-10-18T18:43:00Z'
};
const job: ImportJob = {
	jobId: 'board-job',
	boardName: 'Nailed it!',
	status: 'done',
	phase: 'saving',
	queued: 0,
	running: 0,
	done: 767,
	failed: 0,
	expected: 849,
	listed: 769,
	missing: 80,
	unsupported: 0,
	previouslyDone: 0,
	outcome: 'partial',
	canRetry: true,
	listingIncomplete: false
};
const complete: ImportJob = {
	...job,
	expected: 769,
	missing: 0,
	outcome: 'complete',
	canRetry: false
};
const active: ImportJob = {
	...job,
	status: 'running',
	outcome: '',
	canRetry: false,
	missing: 0,
	queued: 100
};
type Preview = { id: string; name: string; group: string };
type Scenario = Preview & { jobs: ImportJob[]; title: string; action?: string };
const scenarios: Scenario[] = [
	{
		id: 'preparing',
		name: 'Preparing the import',
		group: 'In progress',
		jobs: [],
		title: 'Preparing your import…'
	},
	{
		id: 'reading',
		name: 'Reading boards',
		group: 'In progress',
		jobs: [{ ...active, status: 'listing', done: 0, listed: null }],
		title: 'Reading your boards'
	},
	{
		id: 'checking',
		name: 'Checking a previous import',
		group: 'In progress',
		jobs: [{ ...active, status: 'listing', previouslyDone: 767 }],
		title: '767 images saved'
	},
	{
		id: 'automatic-retry',
		name: 'Pinterest is being retried',
		group: 'In progress',
		jobs: [{ ...active, status: 'listing', phase: 'retrying', done: 0 }],
		title: 'Retrying Pinterest'
	},
	{
		id: 'saving',
		name: 'Saving images',
		group: 'In progress',
		jobs: [active],
		title: '767 images saved'
	},
	{
		id: 'waiting',
		name: 'Waiting for upload limits',
		group: 'In progress',
		jobs: [{ ...active, phase: 'waiting' }],
		title: '767 images saved'
	},
	{
		id: 'reconnect',
		name: 'Account reconnection needed',
		group: 'In progress',
		jobs: [{ ...active, phase: 'reconnect' }],
		title: '767 images saved',
		action: 'Reconnect account'
	},
	{
		id: 'complete',
		name: 'Import complete',
		group: 'Results',
		jobs: [complete],
		title: '767 images saved'
	},
	{
		id: 'partial',
		name: 'Some pins are missing',
		group: 'Results',
		jobs: [job],
		title: '767 images saved',
		action: 'Retry missing images'
	},
	{
		id: 'upload-errors',
		name: 'Some uploads failed',
		group: 'Results',
		jobs: [{ ...complete, failed: 3, outcome: 'partial', canRetry: true }],
		title: '767 images saved',
		action: 'Retry missing images'
	},
	{
		id: 'unsupported',
		name: 'Some pins have no supported image',
		group: 'Results',
		jobs: [{ ...complete, unsupported: 3, outcome: 'partial' }],
		title: '767 images saved'
	},
	{
		id: 'unsupported-only',
		name: 'No supported images',
		group: 'Results',
		jobs: [{ ...complete, done: 0, unsupported: 769, outcome: 'failed' }],
		title: '0 images saved'
	},
	{
		id: 'failed',
		name: 'The import could not finish',
		group: 'Results',
		jobs: [{ ...job, status: 'failed', done: 0, listed: 0, missing: 849, listingIncomplete: true }],
		title: "We couldn't import your pins",
		action: 'Try again'
	},
	{
		id: 'legacy',
		name: 'An older import needs checking',
		group: 'Results',
		jobs: [{ ...job, done: 25, listed: null, missing: 824 }],
		title: '25 images saved',
		action: 'Check for missing images'
	},
	{
		id: 'legacy-section',
		name: 'Older section, no expected total',
		group: 'Results',
		jobs: [{ ...complete, done: 25, listed: null, expected: 0, canRetry: true }],
		title: '25 images saved',
		action: 'Check for missing images'
	},
	{
		id: 'empty',
		name: 'Empty boards',
		group: 'Results',
		jobs: [{ ...complete, done: 0, listed: 0, expected: 0 }],
		title: '0 images saved'
	},
	{
		id: 'repair-complete',
		name: 'A retry recovered more images',
		group: 'Results',
		jobs: [{ ...complete, previouslyDone: 25 }],
		title: '767 images saved'
	}
];
const previews = new Map<string, Preview>();

async function mockImport(
	page: Page,
	jobs: ImportJob[] = [job],
	options: {
		history?: (typeof session)[];
		holdStatus?: boolean;
		statusError?: boolean;
		retryError?: boolean;
		removedDestination?: boolean;
		boards?: boolean;
	} = {}
) {
	let release = () => {};
	const gate = new Promise<void>((done) => {
		release = done;
	});
	const state = { jobs, retries: [] as unknown[] };
	await page.route(`${APPVIEW}/**`, async (route) => {
		const request = route.request();
		const path = new URL(request.url()).pathname;
		const json = (value: unknown) =>
			route.fulfill({ contentType: 'application/json', body: JSON.stringify(value) });
		if (path === '/api/import/sessions') return json({ sessions: options.history ?? [session] });
		if (path === `/api/import/sessions/${session.sessionId}/retry`) {
			state.retries.push(request.postDataJSON());
			if (options.retryError) return route.fulfill({ status: 503, body: 'Unavailable' });
			if (options.removedDestination) return json({ retried: 0, skipped: 1 });
			state.jobs = [{ ...active, status: 'listing', phase: 'retrying', previouslyDone: 767 }];
			return json({ retried: 1, skipped: 0 });
		}
		if (path === `/api/import/sessions/${session.sessionId}`) {
			if (options.holdStatus) await gate;
			if (options.statusError) return route.fulfill({ status: 503, body: 'Unavailable' });
			return json({ sessionId: session.sessionId, jobs: state.jobs });
		}
		if (path === `/api/import/sessions/${prior.sessionId}`)
			return json({ sessionId: prior.sessionId, jobs: [{ ...complete, done: 1200 }] });
		if (path === '/api/import/pinterest/boards')
			return json({
				username: 'pinterest-user',
				boards: options.boards
					? [
							{ id: 'nails', name: 'Nailed it!', pinCount: 849, url: '/pinterest-user/nailed-it/' },
							{ id: 'art', name: 'Art', pinCount: 230, url: '/pinterest-user/art/' },
							{ id: 'cozy', name: 'Cozy', pinCount: 12, url: '/pinterest-user/cozy/' }
						]
					: []
			});
		if (path === '/api/me') return json({ did: 'did:plc:test', handle: 'test.bsky.social' });
		if (path === '/api/me/role') return json({ role: 'user' });
		if (path === '/api/features/seen') return json({ seen: [] });
		if (path === '/api/supporter/status')
			return json({ active: true, subscribed: false, colorTrialsLeft: 5 });
		if (path === '/api/moderation/prefs')
			return json({
				porn: 'blur',
				sexual: 'blur',
				nudity: 'blur',
				graphicMedia: 'blur',
				aiGenerated: 'show'
			});
		return json({});
	});
	return { state, release };
}

async function capture(page: Page, preview: Preview, selector = '[data-import-panel]') {
	mkdirSync(previewDir, { recursive: true });
	for (const [size, width, height] of [
		['mobile', 390, 844],
		['desktop', 1280, 900]
	] as const) {
		await page.setViewportSize({ width, height });
		await page.evaluate(() => document.fonts.ready);
		const start = page.getByRole('button', { name: 'Start another import', exact: true });
		if (await start.count()) {
			await expect(start).toHaveClass(/bg-primary/);
			const button = await start.boundingBox();
			const card = await page.locator('[data-import-result]').boundingBox();
			expect(button).not.toBeNull();
			expect(card).not.toBeNull();
			expect(button!.y + button!.height).toBeLessThanOrEqual(card!.y);
			expect(Math.abs(button!.x + button!.width - card!.x - card!.width)).toBeLessThan(2);
		}
		await expect
			.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
			.toBe(true);
		await page
			.locator(selector)
			.screenshot({ path: resolve(previewDir, `${preview.id}-${size}.png`) });
	}
	previews.set(preview.id, { id: preview.id, name: preview.name, group: preview.group });
}

test.use({ reducedMotion: 'reduce' });
for (const scenario of scenarios) {
	test(`${scenario.name}: one summary and at most one recovery action`, async ({ page }) => {
		await mockImport(page, scenario.jobs);
		await page.goto('/import/pinterest');
		const result = page.locator('[data-import-result]');
		await expect(
			result.getByRole('heading', { level: 2, name: scenario.title, exact: true })
		).toBeVisible();
		await expect(result.getByRole('button')).toHaveCount(scenario.action ? 1 : 0);
		if (scenario.action)
			await expect(
				result.getByRole('button', { name: scenario.action, exact: true })
			).toBeVisible();
		await expect(page.getByText('Partially imported', { exact: true })).toHaveCount(0);
		await expect(page.getByText('Nailed it!', { exact: true })).toHaveCount(0);
		await expect(page.getByText('Import history', { exact: true })).toHaveCount(0);
		if (
			scenario.jobs.length > 0 &&
			scenario.jobs.every((j) => j.status === 'done' || j.status === 'failed')
		)
			await expect(result.getByRole('progressbar')).toHaveCount(0);
		await capture(page, scenario);
	});
}

test('one recovery action retries the session in place', async ({ page }) => {
	const { state } = await mockImport(page);
	await page.goto('/import/pinterest');
	await page.getByRole('button', { name: 'Retry missing images', exact: true }).click();
	await expect.poll(() => state.retries).toEqual([{}]);
	await expect(
		page.getByText("Pinterest is taking a little longer. We're trying again automatically.")
	).toBeVisible();
	await expect(page.getByRole('heading', { name: '767 images saved', exact: true })).toBeVisible();
	await expect(page.locator('[data-import-result]').getByRole('button')).toHaveCount(0);
});

test('board details are collapsed and contain no repeated actions', async ({ page }) => {
	await mockImport(page, [
		job,
		{
			...complete,
			jobId: 'section',
			boardName: 'Seasonal › November',
			done: 7,
			expected: 0,
			listed: 7
		}
	]);
	await page.goto('/import/pinterest');
	await page.getByText('View board details', { exact: true }).click();
	await expect(page.getByText('80 pins not returned by Pinterest', { exact: true })).toBeVisible();
	await expect(page.getByText('Seasonal › November', { exact: true })).toBeVisible();
	await expect(page.locator('[data-import-result]').getByRole('button')).toHaveCount(1);
	await capture(page, {
		id: 'board-details',
		name: 'Board details expanded',
		group: 'Details and recovery'
	});
});

test('history choice cards switch imports by full-card click and keyboard', async ({ page }) => {
	await mockImport(page, [job], { history: [session, prior] });
	await page.goto('/import/pinterest');
	await page.getByText('Import history', { exact: true }).click();
	await expect(page.getByRole('radiogroup', { name: 'Import history', exact: true })).toBeVisible();
	const radios = page.locator('[data-import-history] [role="radio"]');
	await expect(radios).toHaveCount(2);
	await expect(radios.first()).toBeChecked();
	await expect(radios.last()).not.toBeChecked();
	const cards = page.locator('[data-import-history] label');
	const border = (element: Element) => getComputedStyle(element).borderColor;
	await expect
		.poll(() => cards.first().evaluate(border))
		.not.toBe(await cards.last().evaluate(border));
	await expect(page.getByText('Viewing', { exact: true })).toHaveCount(0);
	await capture(
		page,
		{ id: 'history', name: 'Import history expanded', group: 'Details and recovery' },
		'[data-import-history]'
	);
	await page
		.locator('[data-import-history] label')
		.last()
		.getByText('@pinterest-user', { exact: true })
		.click();
	await expect(
		page.getByRole('heading', { name: '1,200 images saved', exact: true })
	).toBeVisible();
	await expect(radios.last()).toBeChecked();
	await expect(radios.first()).not.toBeChecked();
	await expect(page.locator('[data-import-history]')).not.toHaveAttribute('open');
	await expect(page.getByText('Import history', { exact: true })).toBeFocused();
	await page.getByText('Import history', { exact: true }).click();
	await radios.last().focus();
	await page.keyboard.press('ArrowUp');
	await expect(radios.first()).toBeChecked();
	await expect(page.locator('[data-import-history]')).not.toHaveAttribute('open');
	await expect(page.getByText('Import history', { exact: true })).toBeFocused();
	await expect(page.getByRole('heading', { name: '767 images saved', exact: true })).toBeVisible();
});

test('a single previous import remains accessible from the new-import form', async ({ page }) => {
	await mockImport(page);
	await page.goto('/import/pinterest');
	await page.getByRole('button', { name: 'Start another import' }).click();
	await expect(page.getByRole('button', { name: 'Load boards' })).toBeVisible();
	await page.getByRole('button', { name: 'View last import' }).click();
	await expect(page.getByRole('heading', { name: '767 images saved', exact: true })).toBeVisible();
});

test('new users see the introduction and board selection', async ({ page }) => {
	await mockImport(page, [], { history: [], boards: true });
	await page.goto('/import/pinterest');
	await expect(page.getByRole('button', { name: 'Load boards' })).toBeVisible();
	await capture(page, { id: 'start', name: 'First import', group: 'Getting started' }, 'main');
	await page.getByLabel('Pinterest username or URL').fill('pinterest-user');
	await page.getByRole('button', { name: 'Load boards' }).click();
	await expect(page.getByRole('button', { name: 'Import 3 boards' })).toBeVisible();
	await capture(
		page,
		{ id: 'choose-boards', name: 'Choosing boards', group: 'Getting started' },
		'main'
	);
});

test('no public boards is explained', async ({ page }) => {
	await mockImport(page, [], { history: [] });
	await page.goto('/import/pinterest');
	await page.getByLabel('Pinterest username or URL').fill('pinterest-user');
	await page.getByRole('button', { name: 'Load boards' }).click();
	await expect(page.getByText(/No public boards found/)).toBeVisible();
	await capture(
		page,
		{ id: 'no-boards', name: 'No public boards', group: 'Getting started' },
		'main'
	);
});

test('loading and status errors do not claim the import has stopped', async ({ page }) => {
	const { release } = await mockImport(page, [job], { holdStatus: true, statusError: true });
	await page.goto('/import/pinterest');
	await expect(
		page.getByRole('heading', { name: 'Loading your import…', exact: true })
	).toBeVisible();
	await capture(page, { id: 'loading', name: 'Loading an import', group: 'Details and recovery' });
	release();
	await expect(page.getByRole('button', { name: 'Reload status', exact: true })).toBeVisible();
	await expect(page.getByText('Your import may still be running in the background.')).toBeVisible();
	await capture(page, {
		id: 'load-error',
		name: 'Status could not be loaded',
		group: 'Details and recovery'
	});
});

for (const errorCase of ['retry-error', 'removed-destination'] as const) {
	test(errorCase, async ({ page }) => {
		await mockImport(page, [job], {
			retryError: errorCase === 'retry-error',
			removedDestination: errorCase === 'removed-destination'
		});
		await page.goto('/import/pinterest');
		await page.getByRole('button', { name: 'Retry missing images' }).click();
		if (errorCase === 'retry-error') await expect(page.getByRole('alert')).toBeVisible();
		else
			await expect(
				page.getByText('The collections that need retrying were removed.')
			).toBeVisible();
		if (errorCase === 'removed-destination')
			await expect(page.locator('[data-import-result]').getByRole('button')).toHaveCount(0);
		await capture(page, {
			id: errorCase,
			name: errorCase === 'retry-error' ? 'Retry could not start' : 'A collection was removed',
			group: 'Details and recovery'
		});
	});
}

test.afterAll(() => {
	mkdirSync(previewDir, { recursive: true });
	writeFileSync(
		resolve(previewDir, 'manifest.json'),
		JSON.stringify([...previews.values()], null, 2)
	);
});
