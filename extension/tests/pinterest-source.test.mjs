import { afterEach, test } from 'node:test';
import assert from 'node:assert/strict';
import {
	externalSourceURL,
	pinterestPinUrl,
	pinterestHtmlSource,
	resolvePinterestSource
} from '../src/lib/pinterest-source.ts';
const pin = 'https://www.pinterest.com/pin/111/';
const source = 'https://artist.example/work?image=1&size=large';
const html = `<meta property="og:url" content="${pin}"><meta content="${source.replace('&', '&amp;')}" property="og:see_also">`;
const originalFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = originalFetch;
});
test('external sources exclude Pinterest, short links and non-web schemes', () => {
	assert.equal(externalSourceURL(source), source);
	for (const url of [
		pin,
		'https://pinterest.co.uk/pin/1/',
		'https://pin.it/123',
		'javascript:alert(1)',
		'/relative',
		undefined
	])
		assert.equal(externalSourceURL(url), undefined);
});
test('pin URLs require a Pinterest host and an actual pin ID', () => {
	assert.equal(pinterestPinUrl(pin + '?from=feed'), pin);
	for (const url of [
		'https://example.com/pin/111/',
		'https://www.pinterest.com/search/',
		'https://www.pinterest.com/pin/not-a-pin/'
	])
		assert.equal(pinterestPinUrl(url), undefined);
});
test('public page metadata resolves the exact pin and decodes URL separators', () => {
	assert.equal(pinterestHtmlSource(html, pin), source);
	assert.equal(
		pinterestHtmlSource(html, pin.replace('www.pinterest.com', 'it.pinterest.com')),
		source
	);
	assert.equal(
		pinterestHtmlSource(`<a data-test-id="clickthrough-link" href="${source}">Visit site</a>`, pin),
		source
	);
	assert.equal(pinterestHtmlSource(html, 'https://www.pinterest.com/pin/222/'), undefined);
	assert.equal(
		pinterestHtmlSource(html.replace(source.replace('&', '&amp;'), pin), pin),
		undefined
	);
});
test('public source lookup omits credentials and tolerates inaccessible pins', async () => {
	globalThis.fetch = async (url, options) => {
		assert.equal(url, pin);
		assert.equal(options.credentials, 'omit');
		return new Response(html);
	};
	assert.equal(await resolvePinterestSource(pin), source);
	globalThis.fetch = async () => new Response('', { status: 403 });
	assert.equal(await resolvePinterestSource(pin), undefined);
	globalThis.fetch = async () => {
		throw new Error('Offline');
	};
	assert.equal(await resolvePinterestSource(pin), undefined);
	globalThis.fetch = async () => assert.fail('Unexpected request');
	assert.equal(await resolvePinterestSource('https://other.example/pin/111/'), undefined);
});
