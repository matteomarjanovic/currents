import type { CollectionView } from '$lib/types';

export type OrganizeCollectionSort = 'name' | 'recent';

const byName = (a: CollectionView, b: CollectionView) =>
	a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });

export function orderCollections(
	items: CollectionView[],
	sort: OrganizeCollectionSort
): CollectionView[] {
	return [...items].sort((a, b) => {
		const pinOrder = Number(!!b.viewer?.pinned) - Number(!!a.viewer?.pinned);
		if (pinOrder) return pinOrder;
		if (sort === 'recent') {
			const ta = a.lastSavedAt ?? a.createdAt ?? '';
			const tb = b.lastSavedAt ?? b.createdAt ?? '';
			if (ta !== tb) return ta < tb ? 1 : -1;
		}
		return byName(a, b);
	});
}
