<script lang="ts">
	import { ContextMenu as ContextMenuPrimitive } from 'bits-ui';
	import { cn } from '$lib/utils.js';
	import ContextMenuPortal from './context-menu-portal.svelte';
	import type { ComponentProps } from 'svelte';
	import type { WithoutChildrenOrChild } from '$lib/utils.js';
	import MenuDismissSurface from '$lib/components/ui/menu-dismiss-surface.svelte';

	let {
		ref = $bindable(null),
		portalProps,
		class: className,
		child: customChild,
		children,
		...restProps
	}: ContextMenuPrimitive.ContentProps & {
		portalProps?: WithoutChildrenOrChild<ComponentProps<typeof ContextMenuPortal>>;
	} = $props();
</script>

<ContextMenuPortal {...portalProps}>
	<ContextMenuPrimitive.Content
		bind:ref
		data-slot="context-menu-content"
		class={cn(
			'relative z-50 min-w-48 animate-none! overflow-x-hidden overflow-y-auto rounded-3xl bg-popover/70 p-1.5 text-popover-foreground shadow-lg ring-1 ring-foreground/5 duration-100 outline-none before:pointer-events-none before:absolute before:inset-0 before:-z-1 before:rounded-[inherit] before:backdrop-blur-2xl before:backdrop-saturate-150 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 **:data-[slot$=-item]:focus:bg-foreground/10 **:data-[slot$=-item]:data-highlighted:bg-foreground/10 **:data-[slot$=-separator]:bg-foreground/5 **:data-[slot$=-trigger]:focus:bg-foreground/10 **:data-[slot$=-trigger]:aria-expanded:bg-foreground/10! **:data-[variant=destructive]:**:text-accent-foreground! **:data-[variant=destructive]:text-accent-foreground! **:data-[variant=destructive]:focus:bg-foreground/10! dark:ring-foreground/10 data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95',
			className
		)}
		{...restProps}
	>
		{#snippet child({ props, wrapperProps, open })}
			<MenuDismissSurface
				{open}
				onDismiss={() =>
					ref?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))}
			/>
			{#if customChild}
				{@render customChild({ props, wrapperProps, open })}
			{:else}
				<div {...wrapperProps}>
					<div {...props}>{@render children?.()}</div>
				</div>
			{/if}
		{/snippet}
	</ContextMenuPrimitive.Content>
</ContextMenuPortal>
