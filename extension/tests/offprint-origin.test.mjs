import { afterEach, test } from 'node:test';
import assert from 'node:assert/strict';
import { offprintPageOrigin, resolveOffprintOrigin } from '../src/lib/offprint-origin.ts';
const source = 'https://writer.offprint.app/a/first-article';
const image = 'https://cdn.offprint.app/p/21451/image.png';
const ref = {
	uri: 'at://did:plc:pgjkomf37an4czloay5zeth6/site.standard.document/3my6obmsl6t23',
	cid: 'bafyreibjkx4dpzdl4vohkpkj5pxef4sdbkga7mmkw3ai3kfnya2ju6bbbq'
};
const media = {
	url: image,
	medium_url: 'https://cdn.offprint.app/p/21451/conversions/image-medium.jpg'
};
const article = {
	atp_uri: ref.uri,
	atp_cid: ref.cid,
	atp_sync_state: 'synced',
	is_published: true,
	is_draft: false,
	content: { type: 'doc', content: [{ type: 'singleImage', attrs: { mediaData: media } }] }
};
const page = { url: '/a/first-article', props: { domain: 'offprint.app', article } };
const expected = { originAtprotoRef: ref, originUrl: source };
const hint = { app: 'offprint', imageUrl: image, pages: [source] };
const originalFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = originalFetch;
});

test('Offprint uses the canonical document URI and record CID, not its secondary article record', () => {
	assert.deepEqual(offprintPageOrigin(page, image, source), expected);
	const wrong = structuredClone(page);
	wrong.props.article.atp_uri = ref.uri.replace(
		'site.standard.document',
		'app.offprint.document.article'
	);
	assert.equal(offprintPageOrigin(wrong, image, source), undefined);
});
test('Offprint CDN conversions match their original media ID', () => {
	assert.deepEqual(offprintPageOrigin(page, media.medium_url, source), expected);
	assert.equal(
		offprintPageOrigin(page, 'https://cdn.offprint.app/p/99999/image.png', source),
		undefined
	);
});
test('published covers resolve without body content', () => {
	const cover = structuredClone(page);
	cover.props.article.content = null;
	cover.props.article.published_cover = { medium: media.medium_url, original: image };
	assert.deepEqual(offprintPageOrigin(cover, image, source), expected);
});
test('image grids and carousels resolve each image through their media metadata', () => {
	for (const type of ['imageGrid', 'imageCarousel']) {
		const gallery = structuredClone(page);
		gallery.props.article.content = {
			type: 'doc',
			content: [
				{
					type,
					attrs: { mediaData: [media, { url: 'https://cdn.offprint.app/p/12345/photo.jpg' }] }
				}
			]
		};
		assert.deepEqual(
			offprintPageOrigin(
				gallery,
				'https://cdn.offprint.app/p/12345/conversions/photo-large.jpg',
				source
			),
			expected
		);
	}
});
test('avatars, link previews and unrelated media maps remain unattributed', () => {
	const other = structuredClone(page);
	other.props.article.link_assets_media_map = [{ url: 'https://example.com/preview.jpg' }];
	other.props.article.content.content.push({
		type: 'linkPreview',
		attrs: { mediaData: { url: 'https://example.com/preview.jpg' } }
	});
	assert.equal(offprintPageOrigin(other, 'https://example.com/preview.jpg', source), undefined);
	assert.equal(
		offprintPageOrigin(
			page,
			'https://cdn.offprint.app/p/33/conversions/avatar-thumbnail.jpg',
			source
		),
		undefined
	);
});
test('unpublished, unsynced, errored and incomplete articles remain unattributed', () => {
	for (const change of [
		{ is_published: false },
		{ is_draft: true },
		{ atp_sync_state: 'pending' },
		{ atp_sync_error: 'Failed' },
		{ atp_cid: null }
	]) {
		const changed = structuredClone(page);
		Object.assign(changed.props.article, change);
		assert.equal(offprintPageOrigin(changed, image, source), undefined);
	}
	assert.equal(offprintPageOrigin(page, image, 'https://writer.offprint.app/a/another'), undefined);
});
test('matched snapshots keep the version rendered by the page without another request', async () => {
	globalThis.fetch = async () => {
		assert.fail('Unexpected request');
	};
	assert.deepEqual(await resolveOffprintOrigin({ ...hint, snapshot: expected }), expected);
});
test('feed cards resolve through public article JSON without credentials', async () => {
	globalThis.fetch = async (url, options) => {
		assert.equal(url, source);
		assert.equal(options.credentials, 'omit');
		return new Response(
			`<script data-page="app" type="application/json">${JSON.stringify(page)}</script>`
		);
	};
	assert.deepEqual(await resolveOffprintOrigin(hint), expected);
	assert.equal(
		await resolveOffprintOrigin({ ...hint, imageUrl: 'https://example.com/other.png' }),
		undefined
	);
});
test('unavailable articles and unsupported hosts never block saving', async () => {
	globalThis.fetch = async () => new Response('', { status: 503 });
	assert.equal(await resolveOffprintOrigin(hint), undefined);
	globalThis.fetch = async () => new Response('<script data-page="app">invalid</script>');
	assert.equal(await resolveOffprintOrigin(hint), undefined);
	globalThis.fetch = async () => {
		assert.fail('Unexpected request');
	};
	assert.equal(
		await resolveOffprintOrigin({ ...hint, pages: ['https://unrelated.example/a/article'] }),
		undefined
	);
});
