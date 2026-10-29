import { browser } from 'wxt/browser';
import type { ImageOrigin, ImageOriginHint } from './image-origin';

// Created once per picked image. Unmounted forms retain no link to new sessions.
export function imageMetadata(hint?: ImageOriginHint) {
	const data = $state<{ alt: string; edited: boolean; origin?: ImageOrigin }>({
		alt: '',
		edited: false
	});
	const ready = hint
		? browser.runtime
				.sendMessage($state.snapshot({ type: 'RESOLVE_IMAGE_ORIGIN', imageOrigin: hint }))
				.then((response) => {
					data.origin = response?.origin;
					if (!data.edited && !data.alt && data.origin?.alt)
						data.alt = data.origin.alt.trim().slice(0, 2000);
				})
				.catch(() => {})
		: Promise.resolve();
	return { data, ready };
}
