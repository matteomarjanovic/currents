import type { CollectionView } from '$lib/types';

function activityTs(collection: CollectionView): number {
	const saved = collection.lastSavedAt ? Date.parse(collection.lastSavedAt) : 0;
	const created = collection.createdAt ? Date.parse(collection.createdAt) : 0;
	return Math.max(saved, created);
}

function byRecentActivity(a: CollectionView, b: CollectionView): number {
	return activityTs(b) - activityTs(a);
}

function appendUnique(target: CollectionView[], seen: Set<string>, collection?: CollectionView) {
	if (!collection || seen.has(collection.uri)) return;
	seen.add(collection.uri);
	target.push(collection);
}

// The fixed Profile/Create rows are rendered separately. A remembered section is
// the only section allowed at this level; every other section stays under its root.
export function orderCollectionSelectorEntries(
	items: CollectionView[],
	lastUsedUri: string
): CollectionView[] {
	const byUri = new Map(items.map((collection) => [collection.uri, collection]));
	const roots = items.filter((collection) => !collection.parentUri).sort(byRecentActivity);
	const lastUsed = byUri.get(lastUsedUri);
	const ordered: CollectionView[] = [];
	const seen = new Set<string>();

	appendUnique(ordered, seen, lastUsed);
	if (lastUsed?.parentUri) appendUnique(ordered, seen, byUri.get(lastUsed.parentUri));
	for (const root of roots) if (root.viewer?.pinned) appendUnique(ordered, seen, root);
	for (const root of roots) appendUnique(ordered, seen, root);

	return ordered;
}

export function orderCollectionSelectorSections(
	items: CollectionView[],
	parentUri: string,
	lastUsedUri: string
): CollectionView[] {
	const sections = items
		.filter((collection) => collection.parentUri === parentUri)
		.sort(byRecentActivity);
	const ordered: CollectionView[] = [];
	const seen = new Set<string>();

	appendUnique(
		ordered,
		seen,
		sections.find((section) => section.uri === lastUsedUri)
	);
	for (const section of sections) if (section.viewer?.pinned) appendUnique(ordered, seen, section);
	for (const section of sections) appendUnique(ordered, seen, section);

	return ordered;
}
