import { describe, expect, it } from 'vitest';
import {
	exploreFeedScope,
	forgetExploreFeed,
	lastExploreFeed,
	peekExploreFeed,
	rememberExploreFeed,
	resumeExploreFeed,
	type ExploreFeedState
} from './explore-feed';

const state: ExploreFeedState = {
	items: [],
	cursor: 'opaque-pagination-cursor',
	error: null,
	position: { y: 1200, anchor: { uri: 'at://image', top: -30 } }
};

describe('Explore navigation cache', () => {
	it('retains independent feeds and remembers the last level', () => {
		const scope = exploreFeedScope('viewer-one', []);
		rememberExploreFeed(scope, 'general', state);
		rememberExploreFeed(scope, 'personal', { ...state, cursor: 'personal-cursor' });
		expect(lastExploreFeed(scope)).toBe('personal');
		expect(peekExploreFeed(scope, 'general')).toEqual(state);
		expect(peekExploreFeed(exploreFeedScope('someone-else', []), 'general')).toBeUndefined();
		expect(lastExploreFeed(scope)).toBe('personal');
		expect(resumeExploreFeed(scope, 'general')).toEqual(state);
		expect(lastExploreFeed(scope)).toBe('general');
		expect(resumeExploreFeed(scope, 'personal')?.cursor).toBe('personal-cursor');
	});

	it('drops cached results and the last level when the viewer or exclusions change', () => {
		const first = exploreFeedScope('viewer-two', ['collection']);
		rememberExploreFeed(first, 'general', state);
		const otherViewer = exploreFeedScope('viewer-three', ['collection']);
		expect(lastExploreFeed(otherViewer)).toBeUndefined();
		expect(resumeExploreFeed(otherViewer, 'general')).toBeUndefined();
		rememberExploreFeed(otherViewer, 'personal', state);
		const otherExclusions = exploreFeedScope('viewer-three', []);
		expect(lastExploreFeed(otherExclusions)).toBeUndefined();
		expect(resumeExploreFeed(otherExclusions, 'personal')).toBeUndefined();
	});

	it('refresh discards only the active feed', () => {
		const scope = exploreFeedScope('viewer-four', []);
		rememberExploreFeed(scope, 'general', state);
		rememberExploreFeed(scope, 'personal', state);
		forgetExploreFeed(scope, 'personal');
		expect(lastExploreFeed(scope)).toBe('personal');
		expect(resumeExploreFeed(scope, 'personal')).toBeUndefined();
		expect(resumeExploreFeed(scope, 'general')).toEqual(state);
	});

	it('treats exclusion ordering as the same scope', () => {
		expect(exploreFeedScope('viewer', ['a', 'b'])).toBe(exploreFeedScope('viewer', ['b', 'a']));
	});
});
