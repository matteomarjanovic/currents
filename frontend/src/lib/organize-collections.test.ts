import { describe, expect, it } from 'vitest';
import { orderCollections } from './organize-collections';
import type { CollectionView } from './types';

const collection = (
	name: string,
	createdAt: string,
	pinned = false,
	lastSavedAt?: string
): CollectionView => ({
	uri: `at://did:plc:test/is.currents.feed.collection/${name.toLowerCase()}`,
	name,
	createdAt,
	lastSavedAt,
	viewer: pinned ? { pinned: true } : undefined
});

describe('orderCollections', () => {
	it('sorts alphabetically while keeping pinned items first', () => {
		const items = [
			collection('Zulu', '2026-01-03T00:00:00Z'),
			collection('Beta', '2026-01-02T00:00:00Z', true),
			collection('Alpha', '2026-01-01T00:00:00Z')
		];
		expect(orderCollections(items, 'name').map((item) => item.name)).toEqual([
			'Beta',
			'Alpha',
			'Zulu'
		]);
	});

	it('sorts pinned and unpinned groups independently by recent activity', () => {
		const items = [
			collection('Old pin', '2026-01-01T00:00:00Z', true),
			collection('Newest', '2026-01-04T00:00:00Z'),
			collection('New pin', '2026-01-02T00:00:00Z', true, '2026-01-03T00:00:00Z'),
			collection('Older', '2026-01-02T00:00:00Z')
		];
		expect(orderCollections(items, 'recent').map((item) => item.name)).toEqual([
			'New pin',
			'Old pin',
			'Newest',
			'Older'
		]);
	});
});
