import { describe, expect, it } from 'vitest';
import { bunnyImageSrcset, bunnyImageUrl } from './image-url';

const image = 'https://cdn.currents.is/img/did:plc:alice/bafkreiabc';

describe('bunnyImageUrl', () => {
	it('adds the force-image parameter and requested transformations', () => {
		expect(
			bunnyImageUrl(image, {
				aspectRatio: '1:1',
				width: 384,
				quality: 78
			})
		).toBe(
			'https://cdn.currents.is/img/did:plc:alice/bafkreiabc?optimizer=image&aspect_ratio=1%3A1&width=384&quality=78'
		);
	});

	it('leaves origin and third-party URLs alone', () => {
		expect(
			bunnyImageUrl('https://api.currents.is/img/did:plc:alice/bafkreiabc', { width: 320 })
		).toBe('https://api.currents.is/img/did:plc:alice/bafkreiabc');
		expect(bunnyImageUrl('https://example.com/image.jpg', { width: 320 })).toBe(
			'https://example.com/image.jpg'
		);
	});
});

describe('bunnyImageSrcset', () => {
	it('does not request widths above the original image', () => {
		expect(bunnyImageSrcset(image, [320, 480, 640, 960], 500, { quality: 80 })).toBe(
			[
				`${image}?optimizer=image&width=320&quality=80 320w`,
				`${image}?optimizer=image&width=480&quality=80 480w`,
				`${image}?optimizer=image&width=500&quality=80 500w`
			].join(', ')
		);
	});

	it('caps large originals at the largest requested variant', () => {
		const srcset = bunnyImageSrcset(image, [640, 960, 1440, 2048], 4000, { quality: 85 });
		expect(srcset).toContain('width=2048&quality=85 2048w');
		expect(srcset).not.toContain('4000w');
	});

	it('does not build a misleading srcset for a non-Bunny image', () => {
		expect(bunnyImageSrcset('https://example.com/image.jpg', [320, 640], 800)).toBe('');
	});
});
