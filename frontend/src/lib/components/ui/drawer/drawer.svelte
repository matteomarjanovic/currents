<script lang="ts">
	import { Drawer as DrawerPrimitive } from 'vaul-svelte';
	import { onBackButton } from '$lib/back-button';

	let {
		shouldScaleBackground = true,
		open = $bindable(false),
		activeSnapPoint = $bindable(null),
		...restProps
	}: DrawerPrimitive.RootProps = $props();

	// Capacitor replaces its default Android back handling once the app installs a
	// listener. Register every open drawer as an overlay so Back dismisses the sheet
	// before app-init falls through to browser history (or exits at the root).
	$effect(() => {
		if (!open) return;
		return onBackButton(() => (open = false));
	});
</script>

<DrawerPrimitive.Root {shouldScaleBackground} bind:open bind:activeSnapPoint {...restProps} />
