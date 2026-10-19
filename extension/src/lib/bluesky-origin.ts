import type { ImageOrigin, BlueskyImageOriginHint } from './image-origin';

// Shared by page detection and the worker; requests only go to known AppViews.
export const BLUESKY_APIS: Record<string, readonly string[]> = {
	'bsky.app': ['https://public.api.bsky.app'],
	'mu.social': ['https://public.api.bsky.app', 'https://api.eurosky.network'],
	'witchsky.app': ['https://public.api.bsky.app'],
	'northsky.app': ['https://api.blacksky.community'],
	'blacksky.community': ['https://api.blacksky.community'],
	'anisota.net': ['https://public.api.bsky.app'],
	'tokimeki.blue': ['https://public.api.bsky.app']
};

interface Embed {
	$type?: string;
	images?: { image: { ref: { $link: string } } }[];
	media?: Embed;
}

// Views for quoted posts use `value`; top-level post views use `record`.
interface Post {
	uri?: string;
	cid?: string;
	record?: { embed?: Embed };
	value?: { embed?: Embed };
}

function findOrigin(value: unknown, hint: BlueskyImageOriginHint): ImageOrigin | undefined {
	if (!value || typeof value !== 'object') return;
	const post = value as Post;
	const embed = (post.record ?? post.value)?.embed;
	const images = embed?.$type === 'app.bsky.embed.recordWithMedia' ? embed.media : embed;
	if (
		post.uri?.startsWith(`at://${hint.did}/app.bsky.feed.post/`) &&
		post.cid &&
		images?.$type === 'app.bsky.embed.images' &&
		images.images?.some((image) => image.image.ref.$link === hint.blobCid)
	) {
		return {
			originAtprotoRef: { uri: post.uri, cid: post.cid },
			originUrl: `https://${hint.client}/profile/${hint.did}/post/${post.uri.split('/').at(-1)}`
		};
	}
	for (const child of Object.values(value)) {
		const origin = findOrigin(child, hint);
		if (origin) return origin;
	}
}

async function get(api: string, method: string, params: URLSearchParams) {
	const response = await fetch(api + '/xrpc/' + method + '?' + params, {
		credentials: 'omit',
		signal: AbortSignal.timeout(5000)
	});
	if (!response.ok) throw new Error(`Bluesky ${response.status}`);
	return response.json();
}

export async function resolveBlueskyOrigin(
	hint: BlueskyImageOriginHint
): Promise<ImageOrigin | undefined> {
	if (!Object.hasOwn(BLUESKY_APIS, hint.client)) return;
	for (const api of BLUESKY_APIS[hint.client]) {
		try {
			const actors = new Map<string, Promise<string>>();
			const uris = await Promise.allSettled(
				hint.posts.map(async (url) => {
					const [, , actor, , rkey] = new URL(url).pathname.split('/');
					const decodedActor = decodeURIComponent(actor);
					if (!actors.has(actor)) {
						actors.set(
							actor,
							decodedActor.startsWith('did:')
								? Promise.resolve(decodedActor)
								: get(
										api,
										'app.bsky.actor.getProfile',
										new URLSearchParams({ actor: decodedActor })
									).then((profile) => profile.did)
						);
					}
					return `at://${await actors.get(actor)}/app.bsky.feed.post/${rkey}`;
				})
			);
			const params = new URLSearchParams();
			for (const result of uris) {
				if (result.status === 'fulfilled' && !params.getAll('uris').includes(result.value)) {
					params.append('uris', result.value);
				}
			}
			if (!params.has('uris')) continue;
			const origin = findOrigin(await get(api, 'app.bsky.feed.getPosts', params), hint);
			if (origin) return origin;
		} catch {
			// Missing/deleted/private posts or an unavailable AppView must not prevent
			// saving the image. Never guess a reference from the page URL alone.
		}
	}
	return undefined;
}
