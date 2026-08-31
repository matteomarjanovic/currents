import { describe, expect, it } from 'vitest';
import { needsImageRecommendation, resolveQuickSaveDestination } from './save-suggestion';

describe('resolveQuickSaveDestination', () => {
	it('recommends outside Explore and uses the persistent choice in last-used mode', () => {
		expect(
			resolveQuickSaveDestination({
				active: false,
				mode: 'recommended',
				recommendedUri: 'recommended',
				lastUsedUri: 'last'
			})
		).toBe('recommended');
		expect(
			resolveQuickSaveDestination({
				active: true,
				mode: 'last-used',
				recommendedUri: 'recommended',
				lastUsedUri: 'last'
			})
		).toBe('last');
	});

	it('recommends each image in recommended mode', () => {
		expect(
			resolveQuickSaveDestination({
				active: true,
				mode: 'recommended',
				sessionUri: 'session',
				recommendedUri: 'recommended',
				lastUsedUri: 'last'
			})
		).toBe('recommended');
	});

	it('recommends until the session has a successful destination', () => {
		const state = {
			active: true,
			mode: 'recommended-then-last-used' as const,
			recommendedUri: 'recommended',
			lastUsedUri: 'last'
		};
		expect(resolveQuickSaveDestination(state)).toBe('recommended');
		expect(resolveQuickSaveDestination({ ...state, sessionUri: 'session' })).toBe('session');
	});

	it('falls back to the last choice when no recommendation exists', () => {
		expect(
			resolveQuickSaveDestination({
				active: true,
				mode: 'recommended',
				lastUsedUri: 'last'
			})
		).toBe('last');
	});
});

describe('needsImageRecommendation', () => {
	it('stops after the first destination only in the hybrid mode', () => {
		expect(
			needsImageRecommendation({
				active: true,
				mode: 'recommended-then-last-used',
				sessionUri: 'session'
			})
		).toBe(false);
		expect(
			needsImageRecommendation({ active: true, mode: 'recommended', sessionUri: 'session' })
		).toBe(true);
	});

	it('requests recommendations outside Explore unless the mode is last-used', () => {
		expect(
			needsImageRecommendation({
				active: false,
				mode: 'recommended-then-last-used',
				sessionUri: 'old-session'
			})
		).toBe(true);
		expect(needsImageRecommendation({ active: false, mode: 'last-used' })).toBe(false);
	});
});
