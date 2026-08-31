import { SvelteMap } from 'svelte/reactivity';
import { apiFetch } from '$lib/api';
import {
	DEFAULT_SAVE_SUGGESTION_MODE,
	needsImageRecommendation,
	type SaveSuggestionMode
} from '$lib/save-suggestion';

const BATCH_SIZE = 50;

export const exploreSaveSuggestions = $state({
	active: false,
	mode: DEFAULT_SAVE_SUGGESTION_MODE,
	sessionUri: undefined as string | undefined,
	bySaveUri: new SvelteMap<string, string | null>()
});

let generation = 0;
let flushTimer: ReturnType<typeof setTimeout> | undefined;
const queued = new Set<string>();
const requested = new Set<string>();

export function startExploreSaveSession(mode: SaveSuggestionMode) {
	generation += 1;
	exploreSaveSuggestions.active = true;
	exploreSaveSuggestions.mode = mode;
	exploreSaveSuggestions.sessionUri = undefined;
	exploreSaveSuggestions.bySaveUri.clear();
	queued.clear();
	requested.clear();
}

export function endExploreSaveSession() {
	generation += 1;
	exploreSaveSuggestions.active = false;
	exploreSaveSuggestions.sessionUri = undefined;
	exploreSaveSuggestions.bySaveUri.clear();
	queued.clear();
	requested.clear();
	if (flushTimer !== undefined) clearTimeout(flushTimer);
	flushTimer = undefined;
}

export function setExploreSaveSuggestionMode(mode: SaveSuggestionMode) {
	if (exploreSaveSuggestions.mode === mode) return;
	exploreSaveSuggestions.mode = mode;
	exploreSaveSuggestions.sessionUri = undefined;
	if (mode === 'last-used') queued.clear();
}

export function recordExploreSaveDestination(uri: string) {
	if (
		!uri ||
		!exploreSaveSuggestions.active ||
		exploreSaveSuggestions.mode !== 'recommended-then-last-used'
	) {
		return;
	}
	exploreSaveSuggestions.sessionUri = uri;
	queued.clear();
}

export function queueExploreSaveSuggestion(saveUri: string) {
	if (!saveUri || requested.has(saveUri) || !needsImageRecommendation(exploreSaveSuggestions)) {
		return;
	}
	queued.add(saveUri);
	if (flushTimer === undefined) flushTimer = setTimeout(flush, 0);
}

async function flush() {
	flushTimer = undefined;
	if (!needsImageRecommendation(exploreSaveSuggestions)) {
		queued.clear();
		return;
	}
	const saveUris = [...queued].slice(0, BATCH_SIZE);
	for (const uri of saveUris) {
		queued.delete(uri);
		requested.add(uri);
	}
	if (queued.size > 0) flushTimer = setTimeout(flush, 0);
	if (saveUris.length === 0) return;

	const requestGeneration = generation;
	let suggestions: Record<string, string> = {};
	try {
		const res = await apiFetch('/api/save-suggestions', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ saveUris })
		});
		if (res.ok) {
			suggestions =
				((await res.json()) as { suggestions?: Record<string, string> }).suggestions ?? {};
		}
	} catch {
		// Best-effort: a failed recommendation falls back to the persistent last-used destination.
	}
	if (requestGeneration !== generation) return;
	for (const uri of saveUris) {
		exploreSaveSuggestions.bySaveUri.set(uri, suggestions[uri] ?? null);
	}
}
