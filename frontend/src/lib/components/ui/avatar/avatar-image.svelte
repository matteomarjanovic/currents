<script lang="ts">
	import { Avatar as AvatarPrimitive } from 'bits-ui';
	import { cn } from '$lib/utils.js';
	import { bunnyImageUrl } from '$lib/image-url';

	let {
		ref = $bindable(null),
		class: className,
		src,
		...restProps
	}: AvatarPrimitive.ImageProps = $props();
	let optimizedSrc = $derived(
		typeof src === 'string'
			? bunnyImageUrl(src, { aspectRatio: '1:1', width: 256, quality: 80 })
			: src
	);
</script>

<AvatarPrimitive.Image
	bind:ref
	src={optimizedSrc}
	data-slot="avatar-image"
	class={cn('aspect-square size-full rounded-full object-cover', className)}
	{...restProps}
/>
