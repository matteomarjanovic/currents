import type { LastSaveRemovalAction } from '$lib/stores/preferences.svelte';

export const saveRemovalDialog = $state({
	open: false,
	count: 1,
	remember: false
});

let resolvePending: ((action: LastSaveRemovalAction | null) => void) | null = null;

export function askLastSaveRemoval(count = 1): Promise<LastSaveRemovalAction | null> {
	resolvePending?.(null);
	saveRemovalDialog.count = count;
	saveRemovalDialog.remember = false;
	return new Promise((resolve) => {
		resolvePending = resolve;
		saveRemovalDialog.open = true;
	});
}

export function answerLastSaveRemoval(action: LastSaveRemovalAction | null) {
	saveRemovalDialog.open = false;
	resolvePending?.(action);
	resolvePending = null;
}
