<script lang="ts">
	import { AlertDialog as AlertDialogPrimitive } from "bits-ui";
	import AlertDialogPortal from "./alert-dialog-portal.svelte";
	import AlertDialogOverlay from "./alert-dialog-overlay.svelte";
	import { cn, type WithoutChild, type WithoutChildrenOrChild } from "$lib/utils.js";
	import type { ComponentProps } from "svelte";

	let {
		ref = $bindable(null),
		class: className,
		size = "default",
		portalProps,
		...restProps
	}: WithoutChild<AlertDialogPrimitive.ContentProps> & {
		size?: "default" | "sm";
		portalProps?: WithoutChildrenOrChild<ComponentProps<typeof AlertDialogPortal>>;
	} = $props();
</script>

<AlertDialogPortal {...portalProps}>
	<AlertDialogOverlay />
	<AlertDialogPrimitive.Content
		bind:ref
		data-slot="alert-dialog-content"
		data-size={size}
		class={cn(
			// Same safe-area cap as dialog-content.svelte: the centered dialog's symmetric gaps
			// must clear the larger OS inset (status bar / home indicator) on native.
			"data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0 data-closed:zoom-out-95 data-open:zoom-in-95 bg-popover text-popover-foreground ring-foreground/5 dark:ring-foreground/10 gap-6 rounded-4xl p-6 shadow-xl ring-1 duration-100 data-[size=default]:max-w-xs data-[size=sm]:max-w-xs data-[size=default]:sm:max-w-md group/alert-dialog-content fixed top-1/2 left-1/2 z-50 grid max-h-[calc(100dvh-2*max(env(safe-area-inset-top),env(safe-area-inset-bottom),1rem))] w-full -translate-x-1/2 -translate-y-1/2 overflow-y-auto outline-none",
			className
		)}
		{...restProps}
	/>
</AlertDialogPortal>
