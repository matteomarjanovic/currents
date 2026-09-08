import { apiFetch } from '$lib/api';
import { resaveWithFallback } from '$lib/resave';
import type {
	LastSaveRemovalAction,
	LastSaveRemovalPreference
} from '$lib/stores/preferences.svelte';

export interface SaveLocation {
	collectionUri: string;
	saveUri: string;
}

// An empty scope means the whole library. A non-empty scope includes only saves
// in those exact collections; Profile/Unsorted is therefore included only in the
// whole-library view.
export function saveLocationsInScope(
	saves: SaveLocation[],
	collectionUris: string[]
): SaveLocation[] {
	if (collectionUris.length === 0) return saves;
	const scope = new Set(collectionUris);
	return saves.filter((save) => scope.has(save.collectionUri));
}

export function isLastCollectionSave(
	saves: SaveLocation[],
	saveUri: string,
	collectionUri: string
): boolean {
	return collectionUri !== '' && !saves.some((save) => save.saveUri !== saveUri);
}

export function removalAction(
	saves: SaveLocation[],
	saveUri: string,
	collectionUri: string,
	preference: LastSaveRemovalPreference
): LastSaveRemovalPreference {
	return isLastCollectionSave(saves, saveUri, collectionUri) ? preference : 'delete';
}

// Moving is deliberately create-first: a failed delete can leave a duplicate, but
// it can never lose the image the user chose to keep.
export async function removeSaveRecord(
	sourceUri: string,
	saveUri: string,
	action: LastSaveRemovalAction
): Promise<string | undefined> {
	let movedSaveUri: string | undefined;
	if (action === 'move-to-profile') {
		const moved = await resaveWithFallback(sourceUri, '');
		if (!moved.ok) throw new Error(`resave: ${moved.status}`);
		movedSaveUri = ((await moved.json()) as { uri: string }).uri;
	}

	const rkey = saveUri.split('/').pop();
	const removed = await apiFetch(`/api/save/${rkey}`, { method: 'DELETE' });
	if (!removed.ok) throw new Error(`delete: ${removed.status}`);
	return movedSaveUri;
}
