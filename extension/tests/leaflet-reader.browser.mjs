// Run after `npm run build`: node tests/leaflet-reader.browser.mjs
import { chromium, expect } from '../../frontend/node_modules/@playwright/test/index.mjs';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
const extension = fileURLToPath(new URL('../.output/chrome-mv3-dev', import.meta.url));
const profile = await mkdtemp(join(tmpdir(), 'currents-leaflet-reader-'));
const did = 'did:plc:qsn6ihzsby2lr2r3sl4z3bgf',
	cid = 'bafyreibjkx4dpzdl4vohkpkj5pxef4sdbkga7mmkw3ai3kfnya2ju6bbbq';
const articles = ['first', 'second'].map((key, i) => ({
	key,
	title: `Article ${i + 1}`,
	uri: `at://${did}/site.standard.document/${key}`,
	url: `https://writer.leaflet.pub/${key}`,
	blob: `blob-${key}`,
	alt: `Description for article ${i + 1}`,
	image: `https://leaflet.pub/api/atproto_images?did=${did}&cid=blob-${key}&width=1200`
}));
const context = await chromium.launchPersistentContext(profile, {
	channel: 'chromium',
	headless: true,
	viewport: { width: 1280, height: 1000 },
	args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`]
});
const saves = [],
	records = [];
try {
	await context.route('**/*', async (route) => {
		const request = route.request(),
			url = new URL(request.url());
		const json = (value) =>
			route.fulfill({ contentType: 'application/json', body: JSON.stringify(value) });
		if (url.protocol === 'chrome-extension:') return route.continue();
		if (url.pathname === '/api/atproto_images')
			return route.fulfill({
				contentType: 'image/svg+xml',
				body: '<svg xmlns="http://www.w3.org/2000/svg" width="320" height="240"><rect width="320" height="240" fill="salmon"/></svg>'
			});
		if (url.hostname === 'leaflet.pub')
			return route.fulfill({
				contentType: 'text/html',
				body: `<html><head><meta name="at:canonical" content="at://${did}/site.standard.document/stale"></head><body>
   <main>${articles.map((a) => `<div class="postItemMedium"><a class="publishedPostLink" href="${a.url}">${a.title}</a><h2 class="postTitle">${a.title}</h2></div>`).join('')}</main>
   <div class="readerPostViewer" aria-hidden="true" hidden></div>
   <script>
    const articles=${JSON.stringify(articles)};
    function openArticle(i){const a=articles[i],viewer=document.querySelector('.readerPostViewer');viewer.hidden=false;viewer.setAttribute('aria-hidden','false');viewer.innerHTML='<div class="readerLeafletPost"><div id="post-page-'+a.uri+'"><header class="postHeader"><h1 class="postTitle">'+a.title+'</h1></header><img id="reader-image" src="'+a.image+'" alt="Untrusted DOM fallback" width="320" height="240"></div></div><button onclick="openArticle(0)">Previous post</button><button onclick="openArticle(1)">Next post</button>';}
    document.querySelectorAll('a.publishedPostLink').forEach((link,i)=>link.onclick=e=>{e.preventDefault();openArticle(i);document.querySelector('main').replaceChildren()});
   </script></body></html>`
			});
		if (url.hostname === 'writer.leaflet.pub') {
			const article = articles.find((a) => a.url === url.href);
			return route.fulfill({
				contentType: 'text/html',
				body: `<link rel="site.standard.document" href="${article.uri}">`
			});
		}
		if (url.hostname === 'plc.directory')
			return json({
				id: did,
				service: [{ id: '#atproto_pds', serviceEndpoint: 'https://pds-test.example' }]
			});
		if (url.pathname.endsWith('getRecord')) {
			const article = articles.find((a) => a.key === url.searchParams.get('rkey'));
			records.push(article?.uri);
			return article
				? json({
						uri: article.uri,
						cid,
						value: {
							$type: 'site.standard.document',
							content: {
								$type: 'pub.leaflet.content',
								pages: [
									{
										blocks: [
											{
												block: {
													$type: 'pub.leaflet.blocks.image',
													image: { ref: { $link: article.blob } },
													alt: article.alt
												}
											}
										]
									}
								]
							}
						}
					})
				: route.fulfill({ status: 404 });
		}
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
				blob: { $type: 'blob', ref: { $link: 'blob' }, mimeType: 'image/svg+xml', size: 100 }
			});
		if (url.pathname === '/api/save') {
			const body = request.postData() ?? '';
			const part = (name) =>
				body.match(new RegExp('name="' + name + '"\\r\\n\\r\\n([\\s\\S]*?)\\r\\n--'))?.[1] ?? '';
			saves.push({
				alt: part('alt'),
				url: part('url'),
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
	const readerUrl = 'https://leaflet.pub/reader/trending';
	await page.goto(readerUrl, { waitUntil: 'domcontentloaded' });
	await page.locator('currents-clipper').waitFor({ state: 'attached' });
	const panel = page.locator('currents-clipper .clipper-view:not([aria-hidden="true"])');
	const send = (message) =>
		worker.evaluate(async (message) => {
			const [tab] = await chrome.tabs.query({ url: 'https://leaflet.pub/*' });
			await chrome.tabs.sendMessage(tab.id, message);
		}, message);
	async function openSingle(article) {
		await page.locator('#reader-image').dispatchEvent('contextmenu');
		await send({ type: 'SHOW_CLIPPER', imgUrl: article.image, originUrl: readerUrl });
	}
	async function save(expected) {
		const before = saves.length;
		await panel.getByRole('button', { name: /Test collection/ }).click();
		await panel.getByRole('button', { name: 'Save to Currents', exact: true }).click();
		await expect.poll(() => saves.length).toBe(before + 1);
		expect(saves.at(-1)).toEqual({
			alt: expected.alt,
			url: `https://leaflet.pub/p/${encodeURIComponent(did)}/${expected.key}`,
			origin: { uri: expected.uri, cid }
		});
		await expect(panel.getByRole('button', { name: 'Saved', exact: true })).toBeVisible();
	}
	async function close() {
		await page
			.locator('currents-clipper')
			.getByRole('button', { name: 'Close', exact: true })
			.click();
	}
	await page.locator('a.publishedPostLink').first().click();
	expect(page.url()).toBe(readerUrl);
	await openSingle(articles[0]);
	await panel.getByRole('button', { name: '+ Add details', exact: true }).click();
	await expect(panel.getByLabel('Alt text', { exact: true })).toHaveValue(articles[0].alt);
	await panel.getByRole('button', { name: 'Done', exact: true }).click();
	await save(articles[0]);
	await close();
	console.log(
		'PASS reader right-click resolves the opened article and its alt without a route change'
	);
	await page.getByRole('button', { name: 'Next post', exact: true }).click();
	expect(page.url()).toBe(readerUrl);
	await page.locator('#reader-image').hover();
	await page.locator('.currents-grid-save').click();
	await panel.getByRole('button', { name: '+ Add details', exact: true }).click();
	await expect(panel.getByLabel('Alt text', { exact: true })).toHaveValue(articles[1].alt);
	await panel.getByRole('button', { name: 'Done', exact: true }).click();
	await save(articles[1]);
	await close();
	console.log('PASS switching reader articles uses the new source for hover saves');
	await page.evaluate(() => {
		const subpage = document.createElement('div');
		subpage.id = 'post-page-subpage-id';
		subpage.append(document.querySelector('#reader-image'));
		document.querySelector('.readerLeafletPost').append(subpage);
	});
	await send({ type: 'SHOW_CLIPPER_MULTI' });
	await panel.locator('button[aria-pressed]').click();
	await expect(panel.getByText('Alt missing', { exact: true })).toHaveCount(0);
	await panel.getByRole('button', { name: 'Next', exact: true }).click();
	await save(articles[1]);
	await close();
	console.log('PASS the multi-image picker resolves reader subpage images');
	await page.evaluate(() => {
		document.querySelector('[id^="post-page-at://"]').id = 'post-page-unknown';
	});
	const before = records.length;
	await openSingle(articles[1]);
	await panel.getByRole('button', { name: '+ Add details', exact: true }).click();
	await expect(panel.getByLabel('Alt text', { exact: true })).toHaveValue('');
	await panel.getByRole('button', { name: 'Done', exact: true }).click();
	await panel.getByRole('button', { name: /Test collection/ }).click();
	const count = saves.length;
	await panel.getByRole('button', { name: 'Save to Currents', exact: true }).click();
	await expect.poll(() => saves.length).toBe(count + 1);
	expect(saves.at(-1).origin).toBeUndefined();
	expect(saves.at(-1).alt).toBe('');
	expect(records.length).toBe(before);
	console.log('PASS unknown reader documents never reuse stale page metadata or guess a source');
	expect(errors).toEqual([]);
} finally {
	await context.close();
	await rm(profile, { recursive: true, force: true });
}
