import { afterEach, test } from 'node:test';
import assert from 'node:assert/strict';
import { resolveBlueskyOrigin } from '../src/lib/bluesky-origin.ts';

const did = 'did:plc:z72i7hdynmk6r22z27h6tvur';
const blobCid = 'bafkreidvywlo2nyrizqs7vy7rl7e4m5hedt75lxq2xcpb4rlljvtyqhck4';
const recordCid = 'bafyreihprzpev5lpm4wkevfteaurguvmtmzq4cmg6guzwxvmlumvfdcsmy';
const uri = `at://${did}/app.bsky.feed.post/3mxu3hsnebc2v`;
const embed = { $type: 'app.bsky.embed.images', images: [{ image: { ref: { $link: blobCid } } }] };
const post = { uri, cid: recordCid, record: { embed } };
const hint = {
	app: 'bluesky',
	client: 'bsky.app',
	did,
	blobCid,
	posts: ['https://bsky.app/profile/bsky.app/post/3mxu3hsnebc2v']
};
const expected = {
	originAtprotoRef: { uri, cid: recordCid },
	originUrl: `https://bsky.app/profile/${did}/post/3mxu3hsnebc2v`
};
const originalFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = originalFetch;
});

function mock(posts) {
	const requests = [];
	globalThis.fetch = async (raw, options) => {
		const url = new URL(raw);
		requests.push(url);
		assert.equal(options.credentials, 'omit');
		assert.equal(url.origin, 'https://public.api.bsky.app');
		return Response.json(url.pathname.endsWith('getProfile') ? { did } : { posts });
	};
	return requests;
}

test('uses the owning post record CID, not its image blob CID, and resolves handles', async () => {
	const requests = mock([post]);
	assert.deepEqual(await resolveBlueskyOrigin(hint), expected);
	assert.equal(requests[0].searchParams.get('actor'), 'bsky.app');
	assert.deepEqual(requests[1].searchParams.getAll('uris'), [uri]);
});

test('quoted images reference the quoted post, not the enclosing post', async () => {
	mock([
		{
			uri: 'at://did:plc:other/app.bsky.feed.post/outer',
			cid: 'other',
			record: {},
			embed: { record: { record: { uri, cid: recordCid, value: { embed } } } }
		}
	]);
	assert.deepEqual(await resolveBlueskyOrigin(hint), expected);
});

test('images accompanying a quote belong to the enclosing recordWithMedia post', async () => {
	mock([{ ...post, record: { embed: { $type: 'app.bsky.embed.recordWithMedia', media: embed } } }]);
	assert.deepEqual(await resolveBlueskyOrigin(hint), expected);
});

test('does not attribute unrelated images, avatars, or external thumbnails to a post', async () => {
	mock([post]);
	assert.equal(await resolveBlueskyOrigin({ ...hint, blobCid: 'another-image' }), undefined);
	assert.equal(await resolveBlueskyOrigin({ ...hint, did: 'did:plc:another-author' }), undefined);
	mock([
		{
			...post,
			record: {
				embed: {
					$type: 'app.bsky.embed.external',
					external: { thumb: { ref: { $link: blobCid } } }
				}
			}
		}
	]);
	assert.equal(await resolveBlueskyOrigin(hint), undefined);
});

test('DID links need no profile lookup and duplicate candidate URIs are requested once', async () => {
	const requests = mock([post]);
	const url = `https://bsky.app/profile/${did}/post/3mxu3hsnebc2v`;
	assert.deepEqual(await resolveBlueskyOrigin({ ...hint, posts: [url, url] }), expected);
	assert.equal(requests.length, 1);
	assert.deepEqual(requests[0].searchParams.getAll('uris'), [uri]);
});

test('deleted posts, API failures and timeouts leave provenance unknown', async () => {
	mock([]);
	assert.equal(await resolveBlueskyOrigin(hint), undefined);
	globalThis.fetch = async () => new Response('', { status: 503 });
	assert.equal(await resolveBlueskyOrigin(hint), undefined);
	globalThis.fetch = async () => {
		throw new DOMException('Timed out', 'TimeoutError');
	};
	assert.equal(await resolveBlueskyOrigin(hint), undefined);
});

test('an unavailable candidate handle does not discard a verified candidate', async () => {
	const fetchMock = mock([post]);
	const mockedFetch = globalThis.fetch;
	globalThis.fetch = (url, options) =>
		new URL(url).searchParams.get('actor') === 'unavailable.example'
			? Promise.resolve(new Response('', { status: 404 }))
			: mockedFetch(url, options);
	assert.deepEqual(
		await resolveBlueskyOrigin({
			...hint,
			posts: [...hint.posts, 'https://bsky.app/profile/unavailable.example/post/other']
		}),
		expected
	);
	assert.equal(fetchMock.at(-1).pathname, '/xrpc/app.bsky.feed.getPosts');
});

for (const [client, api] of [
	['mu.social', 'https://public.api.bsky.app'],
	['witchsky.app', 'https://public.api.bsky.app'],
	['northsky.app', 'https://api.blacksky.community'],
	['blacksky.community', 'https://api.blacksky.community'],
	['anisota.net', 'https://public.api.bsky.app'],
	['tokimeki.blue', 'https://public.api.bsky.app']
]) {
	test(`${client} resolves post ownership through its AppView and preserves the client URL`, async () => {
		const requests = [];
		globalThis.fetch = async (raw) => {
			const url = new URL(raw);
			requests.push(url);
			assert.equal(url.origin, api);
			return Response.json(url.pathname.endsWith('getProfile') ? { did } : { posts: [post] });
		};
		const origin = await resolveBlueskyOrigin({
			...hint,
			client,
			posts: [`https://${client}/profile/bsky.app/post/3mxu3hsnebc2v`]
		});
		assert.deepEqual(origin, {
			...expected,
			originUrl: expected.originUrl.replace('bsky.app', client)
		});
		assert.deepEqual(requests.at(-1).searchParams.getAll('uris'), [uri]);
	});
}

test('Mu falls back to Eurosky when the post is absent from the Bluesky AppView', async () => {
	const requests = [];
	globalThis.fetch = async (raw) => {
		const url = new URL(raw);
		requests.push(url.origin);
		return Response.json(
			url.pathname.endsWith('getProfile')
				? { did }
				: { posts: url.hostname === 'api.eurosky.network' ? [post] : [] }
		);
	};
	assert.deepEqual(await resolveBlueskyOrigin({ ...hint, client: 'mu.social' }), {
		...expected,
		originUrl: expected.originUrl.replace('bsky.app', 'mu.social')
	});
	assert.deepEqual(requests, [
		'https://public.api.bsky.app',
		'https://public.api.bsky.app',
		'https://api.eurosky.network',
		'https://api.eurosky.network'
	]);
});

test('unknown clients do not trigger network requests', async () => {
	globalThis.fetch = async () => {
		assert.fail('Unexpected request');
	};
	assert.equal(await resolveBlueskyOrigin({ ...hint, client: 'unrelated.example' }), undefined);
});

test('only the matched Bluesky image supplies dedicated alt text, including quotes', async () => {
	const matching = structuredClone(post);
	matching.record.embed.images = [
		{ alt: 'Wrong image alt', image: { ref: { $link: 'another-blob' } } },
		{ alt: 'A red fox asleep in snow', image: { ref: { $link: blobCid } } }
	];
	mock([matching]);
	assert.equal((await resolveBlueskyOrigin(hint)).alt, 'A red fox asleep in snow');
	mock([
		{
			uri: 'outer',
			record: {},
			embed: { record: { record: { uri, cid: recordCid, value: matching.record } } }
		}
	]);
	assert.equal((await resolveBlueskyOrigin(hint)).alt, 'A red fox asleep in snow');
	matching.record.embed.images[1].alt = '  ';
	mock([matching]);
	assert.equal((await resolveBlueskyOrigin(hint)).alt, undefined);
});
