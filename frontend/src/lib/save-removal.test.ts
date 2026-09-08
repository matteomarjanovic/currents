import { describe, expect, it, vi } from 'vitest';
import {
	isLastCollectionSave,
	removalAction,
	saveLocationsInScope,
	type SaveLocation
} from './save-removal';

vi.mock('$lib/api', () => ({ apiFetch: vi.fn() }));
vi.mock('$lib/resave', () => ({ resaveWithFallback: vi.fn() }));

const collectionSave: SaveLocation = {
	collectionUri: 'at://did:plc:me/is.currents.graph.collection/one',
	saveUri: 'at://did:plc:me/is.currents.feed.save/one'
};

describe('last collection save removal', () => {
	it('uses the preference when removing the only saved location', () => {
		expect(
			isLastCollectionSave([collectionSave], collectionSave.saveUri, collectionSave.collectionUri)
		).toBe(true);
		expect(
			removalAction([collectionSave], collectionSave.saveUri, collectionSave.collectionUri, 'ask')
		).toBe('ask');
	});

	it('deletes directly when another saved location remains', () => {
		const saves = [
			collectionSave,
			{ collectionUri: '', saveUri: 'at://did:plc:me/is.currents.feed.save/profile' }
		];
		expect(
			removalAction(saves, collectionSave.saveUri, collectionSave.collectionUri, 'move-to-profile')
		).toBe('delete');
	});

	it('does not intercept deletion from the profile', () => {
		const profileSave = {
			collectionUri: '',
			saveUri: 'at://did:plc:me/is.currents.feed.save/profile'
		};
		expect(removalAction([profileSave], profileSave.saveUri, '', 'ask')).toBe('delete');
	});
});

describe('save removal scope', () => {
	const other = {
		collectionUri: 'at://did:plc:me/is.currents.graph.collection/two',
		saveUri: 'at://did:plc:me/is.currents.feed.save/two'
	};
	const profile = { collectionUri: '', saveUri: 'at://did:plc:me/is.currents.feed.save/profile' };
	const saves = [collectionSave, other, profile];

	it('includes every location for a whole-library search', () => {
		expect(saveLocationsInScope(saves, [])).toEqual(saves);
	});

	it('keeps only memberships in the selected search collections', () => {
		expect(saveLocationsInScope(saves, [other.collectionUri])).toEqual([other]);
		expect(
			saveLocationsInScope(saves, [collectionSave.collectionUri, other.collectionUri])
		).toEqual([collectionSave, other]);
	});
});
