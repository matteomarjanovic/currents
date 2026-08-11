import { describe, it, expect } from 'vitest';
import { selectedSaves, selectableUris, distinctBlobCids, saveRkeys } from './organize-bulk';
import type { SaveView } from './types';

// Minimal image save; `cid` populates the image content's blobCid.
function img(uri: string, cid = 'cid-' + uri): SaveView {
	return {
		uri,
		content: { $type: 'is.currents.content.defs#imageView', blobCid: cid }
	} as unknown as SaveView;
}
// A save whose content isn't an image (unsupported) — never selectable/labelable.
function other(uri: string): SaveView {
	return { uri, content: { $type: 'is.currents.content.defs#other' } } as unknown as SaveView;
}

describe('selectedSaves', () => {
	it('keeps only selected items, in list order', () => {
		const items = [img('a'), img('b'), img('c')];
		expect(selectedSaves(items, new Set(['c', 'a'])).map((s) => s.uri)).toEqual(['a', 'c']);
	});
	it('drops selected uris no longer in the list', () => {
		expect(selectedSaves([img('a')], new Set(['a', 'gone'])).map((s) => s.uri)).toEqual(['a']);
	});
});

describe('selectableUris', () => {
	it('excludes non-image content', () => {
		expect(selectableUris([img('a'), other('b'), img('c')])).toEqual(['a', 'c']);
	});
});

describe('distinctBlobCids', () => {
	it('dedupes by blob cid', () => {
		const saves = [img('a', 'x'), img('b', 'x'), img('c', 'y')];
		expect(distinctBlobCids(saves).sort()).toEqual(['x', 'y']);
	});
	it('skips saves with no image content', () => {
		expect(distinctBlobCids([other('a'), img('b', 'z')])).toEqual(['z']);
	});
});

describe('saveRkeys', () => {
	it('extracts the trailing rkey and skips non-images', () => {
		const saves = [img('at://did/coll/rk1'), other('at://did/coll/rk2')];
		expect(saveRkeys(saves)).toEqual(['rk1']);
	});
});
