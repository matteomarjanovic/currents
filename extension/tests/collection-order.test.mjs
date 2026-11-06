import { test } from 'node:test';
import assert from 'node:assert/strict';
import { orderedRoots, orderedSections } from '../src/lib/collection-order.ts';

const collection = (uri, extra = {}) => ({ uri, name: uri, saveCount: 0, ...extra });
const pinned = collection('pinned', { viewer: { pinned: true }, lastSavedAt: '2026-11-05' });
const last = collection('last', { lastSavedAt: '2026-11-01' });
const recent = collection('recent', { lastSavedAt: '2026-11-06' });
const uris = (items) => items.map((item) => item.uri);

test('last-used root comes before pinned roots and newer activity', () => {
	assert.deepEqual(uris(orderedRoots([recent, pinned, last], last.uri)), [
		'last',
		'pinned',
		'recent'
	]);
	assert.deepEqual(uris(orderedRoots([recent, pinned, last], pinned.uri)), [
		'pinned',
		'recent',
		'last'
	]);
});

test('last-used section and its parent lead pinned roots without duplicates', () => {
	const section = collection('section', { parentUri: pinned.uri });
	assert.deepEqual(uris(orderedRoots([recent, pinned, section, last], section.uri)), [
		'section',
		'pinned',
		'recent',
		'last'
	]);
});

test('missing or unsorted destinations retain pinned-then-activity ordering', () => {
	for (const destination of ['', 'deleted'])
		assert.deepEqual(uris(orderedRoots([recent, last, pinned], destination)), [
			'pinned',
			'recent',
			'last'
		]);
});

test('last-used section leads pinned siblings and never includes other parents', () => {
	const sections = [
		collection('pinned-section', { parentUri: 'parent', viewer: { pinned: true } }),
		collection('last-section', { parentUri: 'parent' }),
		collection('other-section', { parentUri: 'other' })
	];
	assert.deepEqual(uris(orderedSections(sections, 'parent', 'last-section')), [
		'last-section',
		'pinned-section'
	]);
	assert.deepEqual(uris(orderedSections(sections, 'parent', 'pinned-section')), [
		'pinned-section',
		'last-section'
	]);
});
