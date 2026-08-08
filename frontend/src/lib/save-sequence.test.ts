import { describe, it, expect } from 'vitest';
import { neighbourOf, countAfter, appendUnseen } from './save-sequence';
import type { SaveView } from './types';

function run(...uris: string[]): SaveView[] {
	return uris.map((uri) => ({ uri }) as SaveView);
}

describe('neighbourOf', () => {
	it('walks forwards and backwards through the run', () => {
		const r = run('a', 'b', 'c');
		expect(neighbourOf(r, 'b', 1)?.uri).toBe('c');
		expect(neighbourOf(r, 'b', -1)?.uri).toBe('a');
	});

	it('is null at either end', () => {
		const r = run('a', 'b', 'c');
		expect(neighbourOf(r, 'a', -1)).toBeNull();
		expect(neighbourOf(r, 'c', 1)).toBeNull();
	});

	it('is null for a save that was not opened from a grid', () => {
		expect(neighbourOf(run('a', 'b'), 'deep-linked', 1)).toBeNull();
	});

	it('is null with no run at all', () => {
		expect(neighbourOf([], 'a', 1)).toBeNull();
	});
});

describe('countAfter', () => {
	it('counts what is left of the run', () => {
		const r = run('a', 'b', 'c');
		expect(countAfter(r, 'a')).toBe(2);
		expect(countAfter(r, 'c')).toBe(0);
	});

	it('is -1 outside the run, telling it apart from standing on the last image', () => {
		expect(countAfter(run('a', 'b'), 'deep-linked')).toBe(-1);
	});
});

describe('appendUnseen', () => {
	it('adds the newly loaded tail without duplicating what is already held', () => {
		const r = appendUnseen(run('a', 'b'), run('a', 'b', 'c', 'd'));
		expect(r.map((s) => s.uri)).toEqual(['a', 'b', 'c', 'd']);
	});

	it('keeps images that have since dropped out of the grid', () => {
		// The viewer may still be standing on one, and yanking it would strand the run.
		const r = appendUnseen(run('a', 'b'), run('c'));
		expect(r.map((s) => s.uri)).toEqual(['a', 'b', 'c']);
	});

	it('returns the very same run when there is nothing new, so state does not churn', () => {
		const before = run('a', 'b');
		expect(appendUnseen(before, run('a', 'b'))).toBe(before);
		expect(appendUnseen(before, [])).toBe(before);
	});
});
