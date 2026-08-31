export type SaveSuggestionMode = 'last-used' | 'recommended' | 'recommended-then-last-used';

export const DEFAULT_SAVE_SUGGESTION_MODE: SaveSuggestionMode = 'recommended-then-last-used';

interface SuggestionState {
	active: boolean;
	mode: SaveSuggestionMode;
	sessionUri?: string;
}

export function needsImageRecommendation(state: SuggestionState): boolean {
	if (state.mode === 'last-used') return false;
	return state.mode === 'recommended' || !state.active || !state.sessionUri;
}

export function resolveQuickSaveDestination(
	state: SuggestionState & { recommendedUri?: string; lastUsedUri?: string }
): string | undefined {
	if (state.mode === 'last-used') return state.lastUsedUri;
	if (state.mode === 'recommended-then-last-used' && state.active && state.sessionUri) {
		return state.sessionUri;
	}
	return state.recommendedUri ?? state.lastUsedUri;
}
