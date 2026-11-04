import type { FeedLevelSlug } from './feed-levels';
import type { SaveView } from './types';

export interface FeedPosition {
	y: number;
	anchor?: { uri: string; top: number };
}

export interface ExploreFeedState {
	items: SaveView[];
	cursor?: string;
	error: unknown;
	position: FeedPosition;
}

// Client navigation state only: a reload deliberately starts a fresh feed.
const feeds = new Map<FeedLevelSlug, ExploreFeedState>();
let scope: string | undefined;
let lastLevel: FeedLevelSlug | undefined;

export function exploreFeedScope(viewerDID: string, excludedCollections: string[]): string {
	return JSON.stringify([viewerDID, [...excludedCollections].sort()]);
}

function useScope(next: string) {
	if (scope === next) return;
	scope = next;
	feeds.clear();
	lastLevel = undefined;
}

export function resumeExploreFeed(nextScope: string, level: FeedLevelSlug) {
	useScope(nextScope);
	lastLevel = level;
	return feeds.get(level);
}

export function peekExploreFeed(nextScope: string, level: FeedLevelSlug) {
	return scope === nextScope ? feeds.get(level) : undefined;
}

export function rememberExploreFeed(
	nextScope: string,
	level: FeedLevelSlug,
	state: ExploreFeedState
) {
	useScope(nextScope);
	lastLevel = level;
	feeds.set(level, state);
}

export function lastExploreFeed(nextScope: string) {
	useScope(nextScope);
	return lastLevel;
}

export function forgetExploreFeed(nextScope: string, level: FeedLevelSlug) {
	useScope(nextScope);
	feeds.delete(level);
}
