import { afterEach, test } from 'node:test';
import assert from 'node:assert/strict';
import { pcktPageOrigin, resolvePcktOrigin } from '../src/lib/pckt-origin.ts';

const source = 'https://writer.pckt.blog/post-one';
const image = 'https://pckt-blog-media.s3.us-east-2.amazonaws.com/images/media-one/photo.png';
const ref = {
	uri: 'at://did:plc:revjuqmkvrw6fnkxppqtszpv/site.standard.document/3mbfqhezge25u',
	cid: 'bafyreidtj7goatymy4whfb3gvcrvlilowwd2p7tppyh4q6del2sdoefdju'
};
const content = {
	type: 'doc',
	content: [
		{ type: 'imageUpload', attrs: { src: image } },
		{
			type: 'website',
			attrs: { src: 'https://example.com', image: 'https://example.com/card.png' }
		}
	]
};
const page = {
	url: '/post-one',
	props: {
		app_url: 'https://pckt.blog',
		post: {
			atproto_uri: ref.uri,
			atproto_cid: ref.cid,
			is_published: true,
			is_syncing_to_atproto: false,
			content: JSON.stringify(content)
		}
	}
};
const expected = { originAtprotoRef: ref, originUrl: source };
const hint = { app: 'pckt', imageUrl: image, pages: [source] };
const originalFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = originalFetch;
});

test('Pckt captures the owning document URI and record CID from its published page snapshot', () => {
	assert.deepEqual(pcktPageOrigin(page, image, source), expected);
});
test('Pckt image conversions match the original media without changing the source URL', () => {
	const converted = image.replace('/photo.png', '/conversions/photo-content.webp');
	assert.deepEqual(pcktPageOrigin(page, converted, source), expected);
});
test('Pckt cover images and nested gallery content are associated with their document', () => {
	const cover =
		'https://pckt-blog-media.s3.us-east-2.amazonaws.com/cover_image/cover-one/photo.png';
	assert.deepEqual(
		pcktPageOrigin(
			{
				...page,
				props: {
					...page.props,
					post: { ...page.props.post, cover_image_original_url: cover, content: '' }
				}
			},
			cover,
			source
		),
		expected
	);
	const gallery = structuredClone(page);
	gallery.props.post.content = JSON.stringify({
		type: 'doc',
		content: [{ type: 'gallery', attrs: { images: [{ src: image }] } }]
	});
	assert.deepEqual(pcktPageOrigin(gallery, image, source), expected);
});
test('Pckt avatars, website previews, unrelated images and unreferenced uploads remain unattributed', () => {
	assert.equal(pcktPageOrigin(page, 'https://example.com/card.png', source), undefined);
	assert.equal(
		pcktPageOrigin(page, 'https://cdn.bsky.app/img/avatar/plain/did:plc:test/blob', source),
		undefined
	);
	const extra = structuredClone(page);
	extra.props.post.images = [{ url: 'https://example.com/unused.png' }];
	assert.equal(pcktPageOrigin(extra, 'https://example.com/unused.png', source), undefined);
});
test('unpublished, syncing, errored and missing record metadata are not used', () => {
	for (const change of [
		{ is_published: false },
		{ is_syncing_to_atproto: true },
		{ atproto_sync_error: 'Upload failed' },
		{ atproto_cid: null },
		{ atproto_uri: 'at://did:plc:test/site.standard.publication/key' }
	]) {
		const changed = structuredClone(page);
		Object.assign(changed.props.post, change);
		assert.equal(pcktPageOrigin(changed, image, source), undefined);
	}
});
test('stale page data after client navigation cannot attribute an image to the previous document', () => {
	assert.equal(pcktPageOrigin(page, image, 'https://writer.pckt.blog/post-two'), undefined);
});
test('an already matched page snapshot retains its record version without another request', async () => {
	globalThis.fetch = async () => {
		assert.fail('Unexpected request');
	};
	assert.deepEqual(await resolvePcktOrigin({ ...hint, snapshot: expected }), expected);
});
test('feed card lookups match the selected image against the linked public article', async () => {
	globalThis.fetch = async (url, options) => {
		assert.equal(url, source);
		assert.equal(options.credentials, 'omit');
		return new Response(
			`<script data-page="app" type="application/json">${JSON.stringify(page)}</script>`
		);
	};
	assert.deepEqual(await resolvePcktOrigin(hint), expected);
	assert.equal(
		await resolvePcktOrigin({ ...hint, imageUrl: 'https://example.com/other.png' }),
		undefined
	);
});
test('failed lookups and non-Pckt pages leave provenance unknown', async () => {
	globalThis.fetch = async () => new Response('', { status: 503 });
	assert.equal(await resolvePcktOrigin(hint), undefined);
	globalThis.fetch = async () => new Response('<script data-page="app">invalid</script>');
	assert.equal(await resolvePcktOrigin(hint), undefined);
	globalThis.fetch = async () => {
		assert.fail('Unexpected request');
	};
	assert.equal(
		await resolvePcktOrigin({ ...hint, pages: ['https://unrelated.example/article'] }),
		undefined
	);
});
