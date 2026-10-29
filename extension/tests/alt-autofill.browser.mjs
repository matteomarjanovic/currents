// Run after `npm run build`: node tests/alt-autofill.browser.mjs
// Uses the monorepo's existing Playwright install; all pages and writes are mocked.
import { chromium, expect } from '../../frontend/node_modules/@playwright/test/index.mjs';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const extension = fileURLToPath(new URL('../.output/chrome-mv3-dev', import.meta.url));
const profile = await mkdtemp(join(tmpdir(), 'currents-alt-'));
const did = 'did:plc:z72i7hdynmk6r22z27h6tvur';
const cid = 'bafyreihprzpev5lpm4wkevfteaurguvmtmzq4cmg6guzwxvmlumvfdcsmy';
const blobs = [
	'bafkreidvywlo2nyrizqs7vy7rl7e4m5hedt75lxq2xcpb4rlljvtyqhck4',
	'bafkreihgzb3tzpi5kvdv6thu4po4qydzb436xfzgbwv6j5eetjonbi2xle'
];
const alts = ['A red fox curled up in snow.', 'Two blue boats beside a wooden dock.'];
const bskyImages = blobs.map(
	(blob) => `https://cdn.bsky.app/img/feed_thumbnail/plain/${did}/${blob}@jpeg`
);
const grainImages = blobs.map(
	(blob) => `https://cdn.grain.social/signature/feed_fullsize/plain/${did}/${blob}`
);
const posts = blobs.map((blob, i) => ({
	uri: `at://${did}/app.bsky.feed.post/post${i}`,
	cid,
	record: {
		embed: {
			$type: 'app.bsky.embed.images',
			images: [{ alt: alts[i], image: { ref: { $link: blob } } }]
		}
	}
}));
const svg =
	'<svg xmlns="http://www.w3.org/2000/svg" width="320" height="240"><rect width="320" height="240" fill="salmon"/></svg>';
const context = await chromium.launchPersistentContext(profile, {
	channel: 'chromium',
	headless: true,
	viewport: { width: 1280, height: 1000 },
	args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`]
});
let gate = Promise.resolve(),
	lookups = 0,
	unavailable = false;
const saves = [];
try {
	await context.route('**/*', async (route) => {
		const request = route.request(),
			url = new URL(request.url());
		const json = (value) =>
			route.fulfill({ contentType: 'application/json', body: JSON.stringify(value) });
		if (url.protocol === 'chrome-extension:') return route.continue();
		if (url.hostname === 'public.api.bsky.app') {
			if (url.pathname.endsWith('getProfile')) return json({ did });
			lookups++;
			await gate;
			return unavailable ? route.fulfill({ status: 503, body: 'Unavailable' }) : json({ posts });
		}
		if (url.pathname.endsWith('social.grain.unspecced.getGallery'))
			return json({
				gallery: {
					title: 'Gallery title',
					items: grainImages.map((image, i) => ({
						uri: `at://${did}/social.grain.photo/photo${i}`,
						cid,
						thumb: image,
						fullsize: image,
						...(i === 0 ? { alt: alts[0] } : { caption: 'Caption fallback' })
					}))
				}
			});
		if (['bsky.app', 'grain.social', 'example.org'].includes(url.hostname)) {
			const images = url.hostname === 'grain.social' ? grainImages : bskyImages;
			return route.fulfill({
				contentType: 'text/html',
				body: `<html><body>${images.map((src, i) => (url.hostname === 'grain.social' ? `<article class="gallery-card"><img id="image${i}" src="${src}" alt="Gallery title, photo ${i + 1} of 2" width="320" height="240"><a href="/profile/test.example/gallery/album">Gallery</a></article>` : `<article data-testid="feedItem-by-test.example"><a href="/profile/test.example/post/post${i}">Post</a><img id="image${i}" src="${src}" alt="Untrusted DOM text" width="320" height="240"></article>`)).join('')}</body></html>`
			});
		}
		if (['cdn.bsky.app', 'cdn.grain.social'].includes(url.hostname))
			return route.fulfill({ contentType: 'image/svg+xml', body: svg });
		if (url.pathname === '/api/me')
			return json({ did, handle: 'test.example', displayName: 'Test' });
		if (url.pathname.endsWith('getActorCollections'))
			return json({
				collections: [
					{
						uri: `at://${did}/is.currents.feed.collection/test`,
						name: 'Test collection',
						saveCount: 0
					}
				]
			});
		if (url.pathname === '/api/blob/upload-token')
			return json({ token: 'test', pdsUrl: 'https://pds-test.example' });
		if (url.pathname.endsWith('uploadBlob'))
			return json({
				blob: { $type: 'blob', ref: { $link: blobs[0] }, mimeType: 'image/svg+xml', size: 100 }
			});
		if (url.pathname === '/api/save') {
			const body = request.postData() ?? '';
			const part = (name) =>
				body.match(new RegExp('name="' + name + '"\\r\\n\\r\\n([\\s\\S]*?)\\r\\n--'))?.[1] ?? '';
			saves.push({
				alt: part('alt'),
				origin: part('originAtprotoRef') ? JSON.parse(part('originAtprotoRef')) : undefined
			});
			return route.fulfill({ status: 302, headers: { location: 'https://currents.is/' } });
		}
		return route.abort();
	});
	const worker = context.serviceWorkers()[0] ?? (await context.waitForEvent('serviceworker'));
	await worker.evaluate(() =>
		chrome.storage.local.set({ saveButtonSettings: { allSites: true, domains: [] } })
	);
	const page = await context.newPage();
	page.setDefaultTimeout(15000);
	const errors = [];
	page.on('pageerror', (e) => errors.push(e.message));
	await page.emulateMedia({ reducedMotion: 'reduce' });
	let host = 'bsky.app';
	const panel = page.locator('currents-clipper .clipper-view:not([aria-hidden="true"])');
	const send = (message) =>
		worker.evaluate(
			async ({ host, ...message }) => {
				const [tab] = await chrome.tabs.query({ url: `https://${host}/*` });
				await chrome.tabs.sendMessage(tab.id, message);
			},
			{ host, ...message }
		);
	async function visit(domain) {
		host = domain;
		await page.goto(`https://${host}/`, { waitUntil: 'domcontentloaded' });
		await page.locator('currents-clipper').waitFor({ state: 'attached' });
		// Firefox clones extension messages; enforce that boundary in Chromium too.
		const session = await context.newCDPSession(page);
		const worlds = [];
		session.on('Runtime.executionContextCreated', ({ context }) => worlds.push(context));
		await session.send('Runtime.enable');
		const world = worlds.find(
			(world) => world.origin === `chrome-extension://${new URL(worker.url()).hostname}`
		);
		expect(world).toBeDefined();
		const result = await session.send('Runtime.evaluate', {
			contextId: world.id,
			expression: `(() => {
				const sendMessage = chrome.runtime.sendMessage.bind(chrome.runtime);
				chrome.runtime.sendMessage = async (message, ...args) =>
					sendMessage(structuredClone(message), ...args);
			})()`
		});
		expect(result.exceptionDetails).toBeUndefined();
		await session.detach();
	}
	async function open(i = 0) {
		await page.locator(`#image${i}`).dispatchEvent('contextmenu');
		await send({
			type: 'SHOW_CLIPPER',
			imgUrl: host === 'grain.social' ? grainImages[i] : bskyImages[i],
			originUrl: `https://${host}/`
		});
		await expect(panel.getByText('Save to', { exact: true })).toBeVisible();
	}
	async function details() {
		await panel.getByRole('button', { name: '+ Add details', exact: true }).click();
		return panel.getByLabel('Alt text', { exact: true });
	}
	async function save(expected) {
		const before = saves.length;
		await panel.getByRole('button', { name: /Test collection/ }).click();
		await panel.getByRole('button', { name: 'Save to Currents', exact: true }).click();
		await expect.poll(() => saves.length).toBe(before + 1);
		expect(saves.at(-1).alt).toBe(expected);
		await expect(panel.getByRole('button', { name: 'Saved', exact: true })).toBeVisible();
	}
	function hold() {
		let release;
		gate = new Promise((resolve) => {
			release = resolve;
		});
		return () => {
			release();
			gate = Promise.resolve();
		};
	}
	await visit('bsky.app');
	await open();
	let field = await details();
	await expect(field).toHaveValue(alts[0]);
	await field.fill('My own description');
	await panel.getByRole('button', { name: 'Done', exact: true }).click();
	const beforeLookups = lookups;
	await save('My own description');
	expect(lookups).toBe(beforeLookups);
	console.log('PASS verified single-image autofill is editable and reuses its record version');
	await open();
	field = await details();
	await expect(field).toHaveValue(alts[0]);
	await field.fill('');
	await panel.getByRole('button', { name: 'Done', exact: true }).click();
	await save('');
	console.log('PASS explicitly cleared alt stays empty when saved');
	let release = hold();
	await open();
	field = await details();
	await field.fill('Typed while loading');
	await field.fill('');
	release();
	await expect.poll(() => panel.getByLabel('Alt text', { exact: true }).inputValue()).toBe('');
	await panel.getByRole('button', { name: 'Done', exact: true }).click();
	await save('');
	console.log('PASS late lookup cannot overwrite typing or clearing');
	release = hold();
	await open();
	const before = saves.length;
	await panel.getByRole('button', { name: /Test collection/ }).click();
	await panel.getByRole('button', { name: 'Save to Currents', exact: true }).click();
	expect(saves.length).toBe(before);
	release();
	await expect.poll(() => saves.length).toBe(before + 1);
	expect(saves.at(-1).alt).toBe(alts[0]);
	console.log('PASS fast save waits for the source alt lookup');
	release = hold();
	const staleLookups = lookups;
	await open();
	await expect.poll(() => lookups).toBeGreaterThan(staleLookups);
	gate = Promise.resolve();
	await open(1);
	field = await details();
	await expect(field).toHaveValue(alts[1]);
	release();
	await expect(field).toHaveValue(alts[1]);
	console.log('PASS old session lookup cannot affect a reopened image');
	await page
		.locator('currents-clipper')
		.getByRole('button', { name: 'Close', exact: true })
		.click();
	await page.locator('#image1').hover();
	await page.locator('.currents-grid-save').click();
	field = await details();
	await expect(field).toHaveValue(alts[1]);
	console.log('PASS hover-button saves receive verified alt text too');
	await send({ type: 'SHOW_CLIPPER_MULTI' });
	for (const tile of await panel.locator('button[aria-pressed]').all()) await tile.click();
	await expect(panel.getByText('Alt missing', { exact: true })).toHaveCount(0);
	await panel.getByRole('button', { name: '+ Add details', exact: true }).nth(0).click();
	await expect(panel.getByLabel('Alt text', { exact: true })).toHaveValue(alts[0]);
	await panel.getByLabel('Alt text', { exact: true }).fill('Edited first image');
	await panel.getByRole('button', { name: 'Done', exact: true }).click();
	await panel.getByRole('button', { name: '+ Add details', exact: true }).nth(1).click();
	await expect(panel.getByLabel('Alt text', { exact: true })).toHaveValue(alts[1]);
	await panel.getByLabel('Alt text', { exact: true }).fill('');
	await panel.getByRole('button', { name: 'Done', exact: true }).click();
	await expect(panel.getByText('Alt missing', { exact: true })).toHaveCount(1);
	await panel.getByRole('button', { name: 'Next', exact: true }).click();
	await panel.getByRole('button', { name: /Test collection/ }).click();
	await panel.getByRole('button', { name: /Save without a collection/ }).click();
	const batch = saves.length;
	await panel.getByRole('button', { name: 'Save to Currents', exact: true }).click();
	await expect.poll(() => saves.length).toBe(batch + 4);
	expect(
		saves
			.slice(batch)
			.map((s) => s.alt)
			.sort()
	).toEqual(['', '', 'Edited first image', 'Edited first image'].sort());
	console.log('PASS multi-image alt remains per image across edits and destinations');
	unavailable = true;
	await open();
	field = await details();
	await expect(field).toHaveValue('');
	await panel.getByRole('button', { name: 'Done', exact: true }).click();
	await save('');
	unavailable = false;
	console.log('PASS unavailable source metadata leaves alt empty without blocking saving');
	await visit('example.org');
	const genericLookups = lookups;
	await open();
	field = await details();
	await expect(field).toHaveValue('');
	expect(lookups).toBe(genericLookups);
	console.log('PASS generic website DOM alt is not copied');
	await visit('grain.social');
	await open(0);
	field = await details();
	await expect(field).toHaveValue(alts[0]);
	await open(1);
	field = await details();
	await expect(field).toHaveValue('');
	await panel.getByRole('button', { name: 'Done', exact: true }).click();
	await save('');
	console.log('PASS Grain uses dedicated photo alt and ignores its DOM caption fallback');
	await send({ type: 'SHOW_CLIPPER_MULTI' });
	for (const tile of await panel.locator('button[aria-pressed]').all()) await tile.click();
	await expect(panel.getByText('Alt missing', { exact: true })).toHaveCount(1);
	await panel.getByRole('button', { name: 'Next', exact: true }).click();
	await panel.getByRole('button', { name: /Test collection/ }).click();
	const grainBatch = saves.length;
	await panel.getByRole('button', { name: 'Save to Currents', exact: true }).click();
	await expect.poll(() => saves.length).toBe(grainBatch + 2);
	expect(
		saves
			.slice(grainBatch)
			.map((save) => save.alt)
			.sort()
	).toEqual(['', alts[0]].sort());
	console.log('PASS untouched multi-image saves use only dedicated source alt');
	expect(errors).toEqual([]);
} finally {
	await context.close();
	await rm(profile, { recursive: true, force: true });
}
