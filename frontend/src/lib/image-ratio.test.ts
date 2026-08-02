import { describe, it, expect } from 'vitest';
import { tileRatio, isCropped, isLongImage } from './image-ratio';

describe('tileRatio', () => {
	it('leaves normal shapes alone', () => {
		expect(tileRatio(1000, 1500)).toEqual({ width: 1000, height: 1500 });
		expect(tileRatio(1600, 900)).toEqual({ width: 1600, height: 900 });
	});

	it('clamps at exactly 1:2 and keeps it', () => {
		expect(tileRatio(500, 1000)).toEqual({ width: 500, height: 1000 });
	});

	it('shortens anything taller than 1:2', () => {
		expect(tileRatio(500, 1001)).toEqual({ width: 500, height: 1000 });
		expect(tileRatio(600, 9000)).toEqual({ width: 600, height: 1200 });
	});

	it('falls back to 3:4 when dimensions are missing or zero', () => {
		expect(tileRatio(undefined, undefined)).toEqual({ width: 3, height: 4 });
		expect(tileRatio(800, undefined)).toEqual({ width: 3, height: 4 });
		expect(tileRatio(0, 1000)).toEqual({ width: 3, height: 4 });
	});
});

describe('isCropped', () => {
	it('is true only past 1:2', () => {
		expect(isCropped(500, 1000)).toBe(false);
		expect(isCropped(500, 1001)).toBe(true);
	});

	it('is false without dimensions', () => {
		expect(isCropped(undefined, undefined)).toBe(false);
	});
});

describe('isLongImage', () => {
	it('is true only past 1:3', () => {
		expect(isLongImage(500, 1500)).toBe(false);
		expect(isLongImage(500, 1501)).toBe(true);
		expect(isLongImage(600, 9000)).toBe(true);
	});

	// The bands differ on purpose: a tile crops before the detail view starts scrolling.
	it('leaves the grid-cropped band contained in the detail view', () => {
		expect(isCropped(500, 1200)).toBe(true);
		expect(isLongImage(500, 1200)).toBe(false);
	});

	it('is false without dimensions', () => {
		expect(isLongImage(undefined, 4000)).toBe(false);
	});
});
