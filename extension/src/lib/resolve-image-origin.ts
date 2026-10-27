import type { ImageOriginHint } from './image-origin';
import { resolveBlueskyOrigin } from './bluesky-origin';
import { resolveGrainOrigin } from './grain-origin';
import { resolvePcktOrigin } from './pckt-origin';
import { resolveOffprintOrigin } from './offprint-origin';
import { resolveLeafletOrigin } from './leaflet-origin';

export function resolveImageOrigin(hint?: ImageOriginHint) {
	switch (hint?.app) {
		case 'bluesky':
			return resolveBlueskyOrigin(hint);
		case 'grain':
			return resolveGrainOrigin(hint);
		case 'pckt':
			return resolvePcktOrigin(hint);
		case 'offprint':
			return resolveOffprintOrigin(hint);
		case 'leaflet':
			return resolveLeafletOrigin(hint);
		default:
			return Promise.resolve(undefined);
	}
}
