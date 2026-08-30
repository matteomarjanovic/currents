import { describe, expect, it } from 'vitest';
import {
	orderCollectionSelectorEntries,
	orderCollectionSelectorSections
} from './collection-selector-order';
import type { CollectionView } from './types';

function collection(
	name: string,
	createdAt: string,
	options: { parent?: CollectionView; pinned?: boolean; lastSavedAt?: string } = {}
): CollectionView {
	return {
		uri: `at://did:plc:test/is.currents.feed.collection/${name.toLowerCase().replaceAll(' ', '-')}`,
		name,
		createdAt,
		lastSavedAt: options.lastSavedAt,
		parentUri: options.parent?.uri,
		viewer: options.pinned ? { pinned: true } : undefined
	};
}

describe('orderCollectionSelectorEntries', () => {
	it('puts the last-used root before pinned and recently active roots', () => {
		const last = collection('Last used', '2026-01-01T00:00:00Z');
		const olderPin = collection('Older pin', '2026-01-02T00:00:00Z', { pinned: true });
		const newerPin = collection('Newer pin', '2026-01-03T00:00:00Z', { pinned: true });
		const recent = collection('Recent', '2026-01-04T00:00:00Z');

		expect(
			orderCollectionSelectorEntries([last, olderPin, newerPin, recent], last.uri).map(
				(item) => item.name
			)
		).toEqual(['Last used', 'Newer pin', 'Older pin', 'Recent']);
	});

	it('puts a last-used section before its parent and excludes other sections', () => {
		const parent = collection('Characters', '2026-01-01T00:00:00Z');
		const lastSection = collection('Alice', '2026-01-02T00:00:00Z', { parent });
		const pinnedSection = collection('Bob', '2026-01-03T00:00:00Z', {
			parent,
			pinned: true
		});
		const pinnedRoot = collection('Pinned root', '2026-01-04T00:00:00Z', { pinned: true });
		const recentRoot = collection('Recent root', '2026-01-05T00:00:00Z');

		expect(
			orderCollectionSelectorEntries(
				[parent, lastSection, pinnedSection, pinnedRoot, recentRoot],
				lastSection.uri
			).map((item) => item.name)
		).toEqual(['Alice', 'Characters', 'Pinned root', 'Recent root']);
	});

	it('starts with pinned roots when there is no remembered collection', () => {
		const old = collection('Old', '2026-01-01T00:00:00Z');
		const pinned = collection('Pinned', '2026-01-02T00:00:00Z', { pinned: true });
		const recent = collection('Recent', '2026-01-03T00:00:00Z');

		expect(
			orderCollectionSelectorEntries([old, pinned, recent], '').map((item) => item.name)
		).toEqual(['Pinned', 'Recent', 'Old']);
	});
});

describe('orderCollectionSelectorSections', () => {
	it('keeps the last-used section first, then pinned sections, then recent sections', () => {
		const parent = collection('Characters', '2026-01-01T00:00:00Z');
		const last = collection('Last', '2026-01-02T00:00:00Z', { parent });
		const pinned = collection('Pinned', '2026-01-03T00:00:00Z', { parent, pinned: true });
		const recent = collection('Recent', '2026-01-04T00:00:00Z', { parent });

		expect(
			orderCollectionSelectorSections([parent, last, pinned, recent], parent.uri, last.uri).map(
				(item) => item.name
			)
		).toEqual(['Last', 'Pinned', 'Recent']);
	});
});
