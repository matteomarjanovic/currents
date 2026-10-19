import { afterEach, test } from 'node:test';
import assert from 'node:assert/strict';
import {
	leafletImageIdentity,
	leafletDocumentUri,
	leafletHtmlDocument,
	resolveLeafletOrigin
} from '../src/lib/leaflet-origin.ts';
const did = 'did:plc:qsn6ihzsby2lr2r3sl4z3bgf';
const blobCid = 'bafkreidcumsmcnzal5w75e4wwkoaw3zw3flj42iy3xsyuzzdur7tjuqoiq';
const uri = `at://${did}/site.standard.document/first`;
const cid = 'bafyreibjkx4dpzdl4vohkpkj5pxef4sdbkga7mmkw3ai3kfnya2ju6bbbq';
const source = 'https://writer.leaflet.pub/first';
const hint = { app: 'leaflet', did, blobCid, uri, pages: [source] };
const image = { $type: 'pub.leaflet.blocks.image', image: { ref: { $link: blobCid } } };
const document = {
	uri,
	cid,
	value: {
		$type: 'site.standard.document',
		content: {
			$type: 'pub.leaflet.content',
			pages: [{ $type: 'pub.leaflet.pages.linearDocument', blocks: [{ block: image }] }]
		}
	}
};
const originalFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = originalFetch;
});
function mock(record = document) {
	const calls = [];
	globalThis.fetch = async (raw, options) => {
		assert.equal(options.credentials, 'omit');
		const url = new URL(raw);
		calls.push(url.href);
		if (url.hostname === 'plc.directory')
			return Response.json({
				id: hint.did,
				service: [{ id: '#atproto_pds', serviceEndpoint: 'https://pds.example' }]
			});
		if (url.hostname === 'pds.example') return Response.json(record);
		return new Response(`<link href="${uri}" rel="site.standard.document">`);
	};
	return calls;
}
test('Leaflet image identity ignores resizing and excludes draft URLs', () => {
	assert.deepEqual(
		leafletImageIdentity(
			`https://blog.example/api/atproto_images?cid=${blobCid}&did=${did}&width=1200&v=1`
		),
		{ did, blobCid }
	);
	assert.equal(
		leafletImageIdentity('https://leaflet.pub/api/resized_images?path=draft'),
		undefined
	);
	assert.equal(leafletDocumentUri(`at://${did}/site.standard.publication/first`), undefined);
});
test('Leaflet metadata supports AT metadata and standard document links in either attribute order', () => {
	assert.equal(leafletHtmlDocument(`<meta content='${uri}' name='at:canonical'>`), uri);
	assert.equal(leafletHtmlDocument(`<link href="${uri}" rel="site.standard.document"/>`), uri);
	assert.equal(
		leafletHtmlDocument(
			`<meta name="at:canonical" content="at://${did}/site.standard.publication/pub">`
		),
		undefined
	);
});
test('Leaflet resolves the source document record CID, not its blob CID', async () => {
	const calls = mock();
	assert.deepEqual(await resolveLeafletOrigin(hint), {
		originAtprotoRef: { uri, cid },
		originUrl: source
	});
	assert.equal(calls.length, 2);
	assert.equal(new URL(calls[1]).searchParams.get('repo'), did);
});
test('feed cards read article metadata and custom domains preserve their web source', async () => {
	const calls = mock();
	const custom = 'https://blog.example/first';
	assert.deepEqual(await resolveLeafletOrigin({ ...hint, uri: undefined, pages: [custom] }), {
		originAtprotoRef: { uri, cid },
		originUrl: custom
	});
	assert.equal(calls.length, 3);
});
test('covers and nested canvas image blocks match while unrelated images do not', async () => {
	const cover = structuredClone(document);
	cover.value.content = undefined;
	cover.value.coverImage = { ref: { $link: blobCid } };
	mock(cover);
	assert.ok(await resolveLeafletOrigin(hint));
	const canvas = structuredClone(document);
	canvas.value.content.pages = [{ $type: 'pub.leaflet.pages.canvas', blocks: [{ block: image }] }];
	mock(canvas);
	assert.ok(await resolveLeafletOrigin(hint));
	assert.equal(await resolveLeafletOrigin({ ...hint, blobCid: 'unrelated' }), undefined);
});
test('legacy pub.leaflet.document records keep their own URI and CID', async () => {
	const legacy = structuredClone(document);
	legacy.uri = uri.replace('site.standard.document', 'pub.leaflet.document');
	legacy.value = { $type: 'pub.leaflet.document', pages: document.value.content.pages };
	mock(legacy);
	assert.deepEqual(await resolveLeafletOrigin({ ...hint, uri: legacy.uri }), {
		originAtprotoRef: { uri: legacy.uri, cid },
		originUrl: source
	});
});
test('avatars, website previews and quoted record metadata cannot acquire document provenance', async () => {
	const wrong = structuredClone(document);
	wrong.value.content.pages = [
		{ block: { $type: 'pub.leaflet.blocks.website', previewImage: { ref: { $link: blobCid } } } }
	];
	wrong.value.avatar = { ref: { $link: blobCid } };
	wrong.value.bskyPostRef = { uri, cid };
	mock(wrong);
	assert.equal(await resolveLeafletOrigin(hint), undefined);
});
test('wrong repositories, records, missing CIDs and non-Leaflet content are rejected', async () => {
	for (const change of [
		{ uri: uri + 'other' },
		{ cid: null },
		{ value: { $type: 'other', content: document.value.content } },
		{
			value: {
				$type: 'site.standard.document',
				content: { $type: 'other', pages: document.value.content.pages }
			}
		}
	]) {
		mock({ ...document, ...change });
		assert.equal(await resolveLeafletOrigin(hint), undefined);
	}
	const calls = mock();
	assert.equal(
		await resolveLeafletOrigin({ ...hint, uri: uri.replace(did, 'did:plc:someoneelse') }),
		undefined
	);
	assert.equal(calls.length, 1);
});
test('DID web identities resolve their PDS and unavailable lookups remain optional', async () => {
	const webDid = 'did:web:writer.example:users:alice';
	const webUri = uri.replace(did, webDid);
	const calls = [];
	globalThis.fetch = async (raw) => {
		calls.push(raw);
		return calls.length === 1
			? Response.json({
					id: webDid,
					service: [{ id: webDid + '#atproto_pds', serviceEndpoint: 'https://pds.example' }]
				})
			: Response.json({ ...document, uri: webUri });
	};
	assert.ok(await resolveLeafletOrigin({ ...hint, did: webDid, uri: webUri }));
	assert.equal(calls[0], 'https://writer.example/users/alice/did.json');
	globalThis.fetch = async () => new Response('', { status: 503 });
	assert.equal(await resolveLeafletOrigin(hint), undefined);
	globalThis.fetch = async () => {
		throw new Error('Offline');
	};
	assert.equal(await resolveLeafletOrigin(hint), undefined);
});
