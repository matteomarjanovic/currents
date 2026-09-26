import type { Collection } from './clipper-store.svelte';

function byActivity(a: Collection, b: Collection): number {
	const activity = (c: Collection) =>
		Math.max(Date.parse(c.lastSavedAt ?? '') || 0, Date.parse(c.createdAt ?? '') || 0);
	return activity(b) - activity(a);
}

function unique(items: (Collection | undefined)[]): Collection[] {
	const seen = new Set<string>();
	return items.filter((item): item is Collection => {
		if (!item || seen.has(item.uri)) return false;
		seen.add(item.uri);
		return true;
	});
}

export function orderedRoots(items: Collection[], lastUsedUri: string): Collection[] {
	const byUri = new Map(items.map((item) => [item.uri, item]));
	const roots = items.filter((item) => !item.parentUri).sort(byActivity);
	const last = byUri.get(lastUsedUri);
	return unique([
		last,
		last?.parentUri ? byUri.get(last.parentUri) : undefined,
		...roots.filter((item) => item.viewer?.pinned),
		...roots
	]);
}

export function orderedSections(
	items: Collection[],
	parentUri: string,
	lastUsedUri: string
): Collection[] {
	const sections = items.filter((item) => item.parentUri === parentUri).sort(byActivity);
	return unique([
		sections.find((item) => item.uri === lastUsedUri),
		...sections.filter((item) => item.viewer?.pinned),
		...sections
	]);
}
