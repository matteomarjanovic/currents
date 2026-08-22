import { apiFetch } from '$lib/api';
import type { FeedLevelSlug } from '$lib/feed-levels';

interface FeedPreferences {
	excludedCollections: string[];
	defaultFeed: FeedLevelSlug;
}

export const feedPreferences = $state<FeedPreferences>({
	excludedCollections: [],
	defaultFeed: 'personal'
});
export const feedPreferencesLoaded = $state({ value: false });

export async function loadFeedPreferences() {
	try {
		const res = await apiFetch('/api/feed/preferences');
		if (!res.ok) return;
		const data = (await res.json()) as Partial<FeedPreferences>;
		if (Array.isArray(data.excludedCollections)) {
			feedPreferences.excludedCollections = data.excludedCollections;
		}
		if (
			data.defaultFeed === 'general' ||
			data.defaultFeed === 'new-worlds' ||
			data.defaultFeed === 'personal'
		) {
			feedPreferences.defaultFeed = data.defaultFeed;
		}
		feedPreferencesLoaded.value = true;
	} catch {
		// Best-effort; retry the next time settings opens.
	}
}

let saveQueue = Promise.resolve();

function persist() {
	const excludedCollections = [...feedPreferences.excludedCollections];
	const defaultFeed = feedPreferences.defaultFeed;
	saveQueue = saveQueue.then(async () => {
		try {
			await apiFetch('/api/feed/preferences', {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ excludedCollections, defaultFeed })
			});
		} catch {
			// Best-effort; the server value is loaded again in a later session.
		}
	});
}

export function setCollectionExcluded(uri: string, excluded: boolean) {
	feedPreferences.excludedCollections = excluded
		? [...feedPreferences.excludedCollections, uri]
		: feedPreferences.excludedCollections.filter((value) => value !== uri);
	persist();
}

export function setDefaultFeed(defaultFeed: FeedLevelSlug) {
	feedPreferences.defaultFeed = defaultFeed;
	persist();
}
