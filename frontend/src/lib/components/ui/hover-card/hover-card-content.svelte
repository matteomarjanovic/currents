<script lang="ts">
	import { LinkPreview as HoverCardPrimitive } from 'bits-ui';
	import { cn } from '$lib/utils';

	let {
		ref = $bindable(null),
		class: className,
		side = 'bottom',
		sideOffset = 4,
		align = 'center',
		...restProps
	}: HoverCardPrimitive.ContentProps = $props();
</script>

<HoverCardPrimitive.Portal>
	<HoverCardPrimitive.Content
		bind:ref
		data-slot="hover-card-content"
		{side}
		{sideOffset}
		{align}
		class={cn(
			'z-50 w-64 origin-(--bits-link-preview-content-transform-origin) rounded-xl border bg-popover p-4 text-sm text-popover-foreground shadow-md outline-none data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95',
			className
		)}
		{...restProps}
	/>
</HoverCardPrimitive.Portal>
