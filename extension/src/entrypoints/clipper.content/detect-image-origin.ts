import { blueskyImageOrigin } from './bluesky-image';
import { grainImageOrigin } from './grain-image';
import { pcktImageOrigin } from './pckt-image';
import { offprintImageOrigin } from './offprint-image';
import { leafletImageOrigin } from './leaflet-image';

export function detectImageOrigin(img: HTMLImageElement) {
	return (
		leafletImageOrigin(img) ??
		offprintImageOrigin(img) ??
		pcktImageOrigin(img) ??
		grainImageOrigin(img) ??
		blueskyImageOrigin(img)
	);
}
