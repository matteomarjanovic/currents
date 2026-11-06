// Run after `npm run build`: node tests/pinterest-source.browser.mjs
// Pages, source lookups, and saves are all mocked.
import { chromium, expect } from '../../frontend/node_modules/@playwright/test/index.mjs';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
const extension = fileURLToPath(new URL('../.output/chrome-mv3-dev', import.meta.url));
const profile = await mkdtemp(join(tmpdir(), 'currents-pinterest-'));
const pinA = 'https://www.pinterest.com/pin/111/',
	pinB = 'https://www.pinterest.com/pin/222/';
const sourceA = 'https://artist.example/first',
	sourceB = 'https://artist.example/second';
const imgA = 'https://i.pinimg.com/originals/one.jpg',
	imgB = 'https://i.pinimg.com/originals/two.jpg';
const context = await chromium.launchPersistentContext(profile, {
	channel: 'chromium',
	headless: true,
	viewport: { width: 1280, height: 1000 },
	args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`]
});
const saves = [];
let unavailable = false;
try {
	await context.route('**/*', async (route) => {
		const req = route.request(),
			url = new URL(req.url());
		const json = (value) =>
			route.fulfill({ contentType: 'application/json', body: JSON.stringify(value) });
		if (url.protocol === 'chrome-extension:') return route.continue();
		if (url.hostname === 'www.pinterest.com') {
			if (unavailable) return route.fulfill({ status: 403, body: 'Unavailable' });
			const first = url.pathname === '/pin/111/';
			return route.fulfill({
				contentType: 'text/html',
				body: `<html><head><meta property="og:url" content="${first ? pinA : pinB}"><meta property="og:see_also" content="${first ? sourceA : sourceB}"></head><body><div data-test-id="pin-closeup-image"><img id="first" src="${imgA}" width="320" height="240"></div><div data-grid-item="true"><div data-test-id="pin" data-test-pin-id="222"><a href="${pinB}">Other pin</a><img id="second" src="${imgB}" width="320" height="240"></div></div></body></html>`
			});
		}
		if (url.hostname === 'i.pinimg.com')
			return route.fulfill({
				contentType: 'image/svg+xml',
				body: '<svg xmlns="http://www.w3.org/2000/svg" width="320" height="240"><rect width="320" height="240" fill="salmon"/></svg>'
			});
		if (url.pathname === '/api/me')
			return json({ did: 'did:plc:test', handle: 'test.example', displayName: 'Test' });
		if (url.pathname.endsWith('getActorCollections'))
			return json({
				collections: [
					{
						uri: 'at://did:plc:test/is.currents.feed.collection/test',
						name: 'Test collection',
						saveCount: 0
					}
				]
			});
		if (url.pathname === '/api/blob/upload-token')
			return json({ token: 'test', pdsUrl: 'https://pds-test.example' });
		if (url.pathname.endsWith('uploadBlob'))
			return json({
				blob: { $type: 'blob', ref: { $link: 'blob' }, mimeType: 'image/svg+xml', size: 100 }
			});
		if (url.pathname === '/api/save') {
			const body = req.postData() ?? '';
			const part = (name) =>
				body.match(new RegExp('name="' + name + '"\\r\\n\\r\\n([\\s\\S]*?)\\r\\n--'))?.[1] ?? '';
			saves.push(part('url'));
			return route.fulfill({ status: 302, headers: { location: 'https://currents.is/' } });
		}
		return route.abort();
	});
	const worker = context.serviceWorkers()[0] ?? (await context.waitForEvent('serviceworker'));
	const page = await context.newPage();
	page.setDefaultTimeout(15000);
	await page.emulateMedia({ reducedMotion: 'reduce' });
	const errors = [];
	page.on('pageerror', (e) => errors.push(e.message));
	await page.goto(pinA, { waitUntil: 'domcontentloaded' });
	await page.locator('currents-clipper').waitFor({ state: 'attached' });
	const panel = page.locator('currents-clipper .clipper-view:not([aria-hidden="true"])');
	const send = (message) =>
		worker.evaluate(async (message) => {
			const [tab] = await chrome.tabs.query({ url: 'https://www.pinterest.com/*' });
			await chrome.tabs.sendMessage(tab.id, message);
		}, message);
	const close = () =>
		page.locator('currents-clipper').getByRole('button', { name: 'Close', exact: true }).click();
	async function open(image) {
		await page.locator(image === imgA ? '#first' : '#second').dispatchEvent('contextmenu');
		await send({ type: 'SHOW_CLIPPER', imgUrl: image, originUrl: pinA });
	}
	async function save(expected) {
		const before = saves.length;
		await panel.getByRole('button', { name: /Test collection/ }).click();
		await panel.getByRole('button', { name: 'Save to Currents', exact: true }).click();
		await expect.poll(() => saves.length).toBe(before + 1);
		expect(saves.at(-1)).toBe(expected);
		await expect(panel.getByRole('button', { name: 'Saved', exact: true })).toBeVisible();
		await close();
	}
	await open(imgA);
	await save(sourceA);
	console.log('PASS right-click hero saves its external source');
	await page.locator('#first').hover();
	await page.locator('[data-test-id="pin-closeup-image"] .currents-grid-save').click();
	await save(sourceA);
	console.log('PASS hero image button uses the same source detection');
	await open(imgB);
	await save(sourceB);
	console.log('PASS related pin resolves its own source instead of the hero source');
	await page.locator('#second').hover();
	await page.locator('[data-test-pin-id="222"] .currents-grid-save').click();
	await save(sourceB);
	console.log('PASS grid image button resolves the owning pin source');
	await send({ type: 'SHOW_CLIPPER_MULTI' });
	for (const tile of await panel.locator('button[aria-pressed]').all()) await tile.click();
	await panel.getByRole('button', { name: 'Next', exact: true }).click();
	await panel.getByRole('button', { name: /Test collection/ }).click();
	const before = saves.length;
	await panel.getByRole('button', { name: 'Save to Currents', exact: true }).click();
	await expect.poll(() => saves.length).toBe(before + 2);
	expect(saves.slice(before).sort()).toEqual([sourceA, sourceB].sort());
	await close();
	console.log('PASS multi-image saves preserve each pin source');
	await page.evaluate(() => document.querySelector('meta[property="og:see_also"]').remove());
	unavailable = true;
	await open(imgA);
	await save(pinA);
	await open(imgB);
	await save(pinB);
	console.log('PASS unavailable sources fall back to each owning pin URL');
	await page.evaluate(
		({ sourceA, sourceB }) => {
			const heroLink = document.createElement('a');
			heroLink.dataset.testId = 'clickthrough-link';
			heroLink.href = sourceA;
			document.body.append(heroLink);
			const gridLink = document.createElement('a');
			gridLink.dataset.testId = 'pinrep-source-link';
			gridLink.href = sourceB;
			document.querySelector('[data-test-pin-id="222"]').append(gridLink);
		},
		{ sourceA, sourceB }
	);
	await open(imgA);
	await save(sourceA);
	await open(imgB);
	await save(sourceB);
	console.log('PASS visible hero and card links work without a public page lookup');
	expect(errors).toEqual([]);
} finally {
	await context.close();
	await rm(profile, { recursive: true, force: true });
}
