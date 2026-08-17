const BUNNY_IMAGE_HOST = 'cdn.currents.is';

export interface ImageTransform {
	width?: number;
	quality?: number;
	aspectRatio?: string;
	crop?: string;
	cropGravity?: 'center' | 'north';
}

export function isBunnyImageUrl(src: string): boolean {
	const url = new URL(src);
	return url.hostname === BUNNY_IMAGE_HOST && url.pathname.startsWith('/img/');
}

export function bunnyImageUrl(src: string, transform: ImageTransform = {}): string {
	const url = new URL(src);
	if (!isBunnyImageUrl(src)) return src;

	url.searchParams.set('optimizer', 'image');
	if (transform.aspectRatio) url.searchParams.set('aspect_ratio', transform.aspectRatio);
	if (transform.crop) url.searchParams.set('crop', transform.crop);
	if (transform.cropGravity) url.searchParams.set('crop_gravity', transform.cropGravity);
	if (transform.width) url.searchParams.set('width', String(transform.width));
	if (transform.quality) url.searchParams.set('quality', String(transform.quality));
	return url.toString();
}

export function bunnyImageSrcset(
	src: string,
	widths: number[],
	originalWidth: number,
	transform: Omit<ImageTransform, 'width'> = {}
): string {
	if (!isBunnyImageUrl(src)) return '';
	const candidates = widths.filter((width) => width < originalWidth);
	if (originalWidth <= Math.max(...widths)) candidates.push(originalWidth);
	return [...new Set(candidates)]
		.map((width) => `${bunnyImageUrl(src, { ...transform, width })} ${width}w`)
		.join(', ');
}
