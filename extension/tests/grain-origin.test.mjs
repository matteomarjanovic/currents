import { afterEach, test } from 'node:test';
import assert from 'node:assert/strict';
import { grainImageIdentity, resolveGrainOrigin } from '../src/lib/grain-origin.ts';

const did = 'did:plc:pko7wbcggok753hnvndxh3ni';
const blobCid = 'bafkreieuesy2byl2rckmy23ruh3nnj6lxfakolqxegr62fyfylfyoidzla';
const photoCid = 'bafyreiayu3lo4watg3zxuo5jy4vpac5wdxziwiebho7cp2v5tsslkethue';
const uri = `at://${did}/social.grain.photo/3myatz6zvt4pa`;
const source = 'https://grain.social/profile/manton.org/gallery/3myatz6zvt4p7';
const image = (blob = blobCid, preset = 'feed_fullsize', owner = did) =>
	`https://cdn.grain.social/signature/${preset}/plain/${owner}/${blob}`;
const photo = { uri, cid: photoCid, thumb: image(blobCid, 'feed_thumbnail'), fullsize: image() };
const hint = { app: 'grain', did, blobCid, galleries: [source] };
const originalFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = originalFetch;
});
function mock(items) {
	globalThis.fetch = async (raw, options) => {
		const url = new URL(raw);
		assert.equal(url.origin, 'https://grain.social');
		assert.equal(url.pathname, '/xrpc/social.grain.unspecced.getGallery');
		assert.equal(
			url.searchParams.get('gallery'),
			'at://manton.org/social.grain.gallery/3myatz6zvt4p7'
		);
		assert.equal(options.credentials, 'omit');
		return Response.json({ gallery: { cid: 'gallery-record-cid', items } });
	};
}
test('Grain references the individual photo record and its CID, not the gallery or blob', async () => {
	mock([photo]);
	assert.deepEqual(await resolveGrainOrigin(hint), {
		originAtprotoRef: { uri, cid: photoCid },
		originUrl: source
	});
});
test('each image in a gallery resolves to its own photo record', async () => {
	const second = {
		...photo,
		uri: `at://${did}/social.grain.photo/second`,
		cid: 'second-record-cid',
		fullsize: image('second-blob'),
		thumb: image('second-blob', 'feed_thumbnail')
	};
	mock([photo, second]);
	assert.equal((await resolveGrainOrigin(hint)).originAtprotoRef.uri, uri);
	assert.deepEqual(
		(await resolveGrainOrigin({ ...hint, blobCid: 'second-blob' })).originAtprotoRef,
		{ uri: second.uri, cid: second.cid }
	);
});
test('signed thumbnails and fullsize images identify the same original blob', () => {
	assert.deepEqual(grainImageIdentity(image()), { did, blobCid });
	assert.deepEqual(
		grainImageIdentity(image(blobCid, 'feed_thumbnail').replace('signature', 'another-signature')),
		{ did, blobCid }
	);
	assert.equal(grainImageIdentity(image(blobCid, 'avatar')), undefined);
	assert.equal(
		grainImageIdentity(image().replace('cdn.grain.social', 'unrelated.example')),
		undefined
	);
});
test('unrelated blobs, authors, and gallery membership records remain unattributed', async () => {
	mock([photo]);
	assert.equal(await resolveGrainOrigin({ ...hint, blobCid: 'unrelated' }), undefined);
	assert.equal(await resolveGrainOrigin({ ...hint, did: 'did:plc:another' }), undefined);
	mock([{ ...photo, uri: `at://${did}/social.grain.gallery.item/item` }]);
	assert.equal(await resolveGrainOrigin(hint), undefined);
});
test('duplicate photo records sharing one blob are ambiguous', async () => {
	mock([photo, { ...photo, uri: `at://${did}/social.grain.photo/another` }]);
	assert.equal(await resolveGrainOrigin(hint), undefined);
});
test('missing photos and unavailable galleries do not block saving', async () => {
	mock([]);
	assert.equal(await resolveGrainOrigin(hint), undefined);
	globalThis.fetch = async () => new Response('', { status: 503 });
	assert.equal(await resolveGrainOrigin(hint), undefined);
	globalThis.fetch = async () => {
		throw new DOMException('Timed out', 'TimeoutError');
	};
	assert.equal(await resolveGrainOrigin(hint), undefined);
});
test('unsupported gallery links do not cause arbitrary network requests', async () => {
	globalThis.fetch = async () => {
		assert.fail('Unexpected request');
	};
	assert.equal(
		await resolveGrainOrigin({
			...hint,
			galleries: [
				'https://unrelated.example/profile/manton.org/gallery/key',
				'https://grain.social/profile/manton.org'
			]
		}),
		undefined
	);
});

test('Grain uses the photo alt field and never a gallery title or caption fallback', async () => {
	mock([{ ...photo, alt: 'A cormorant drying its wings', caption: 'At the lake' }]);
	assert.equal((await resolveGrainOrigin(hint)).alt, 'A cormorant drying its wings');
	for (const alt of [undefined, '', '  ']) {
		mock([{ ...photo, alt, title: 'At the lake', caption: 'At the lake, photo 1 of 2' }]);
		assert.equal((await resolveGrainOrigin(hint)).alt, undefined);
	}
});
