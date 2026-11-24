import { describe, expect, it } from 'vitest';
import { resolveVisibility, type Visibility } from './moderation-visibility';

const bluesky = 'did:plc:ar7c4by46qjdydhdevvrndac';
const currents = 'did:web:moderation.currents.is';
const label = (src: string, val: string, exp?: string) => ({ src, val, exp, cts: '' });
const prefs = (val: string): Visibility => {
	if (val === '!hide') return 'hide';
	if (val === 'porn') return 'blur';
	if (val === 'currents-ai-generated') return 'hide';
	return 'show';
};

describe('combined moderation visibility', () => {
	it('keeps the strongest restriction, regardless of issuer order', () => {
		const labels = [label(currents, '!hide'), label(bluesky, 'porn')];
		expect(resolveVisibility(prefs, true, labels)).toBe('hide');
		expect(resolveVisibility(prefs, true, labels.reverse())).toBe('hide');
		expect(resolveVisibility(prefs, true, [label(bluesky, 'porn')], ['!hide'])).toBe('hide');
	});

	it('uses existing media preferences for a Bluesky label on saves and previews', () => {
		const labels = [label(bluesky, 'porn')];
		expect(resolveVisibility(prefs, true, labels)).toBe('blur');
		expect(resolveVisibility(() => 'hide', true, labels)).toBe('hide');
		expect(resolveVisibility(() => 'show', true, labels)).toBe('show');
	});

	it('honors signed-out restrictions, warnings and label expiration', () => {
		const labels = [label(bluesky, '!no-unauthenticated'), label(bluesky, '!warn')];
		expect(resolveVisibility(prefs, false, labels)).toBe('hide');
		expect(resolveVisibility(prefs, true, labels)).toBe('blur');
		expect(
			resolveVisibility(
				prefs,
				false,
				[label(bluesky, '!hide', '2026-11-24T08:00:00Z')],
				[],
				Date.parse('2026-11-24T08:00:00Z')
			)
		).toBe('show');
		expect(resolveVisibility(prefs, true, [label(bluesky, 'currents-ai-generated')])).toBe('show');
	});
});
