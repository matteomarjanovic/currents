<script lang="ts">
	import { Dialog as DialogPrimitive } from "bits-ui";
	import { onBackButton } from "$lib/back-button";

	let { open = $bindable(false), ...restProps }: DialogPrimitive.RootProps = $props();

	// Android's back button dismisses the dialog instead of navigating away from the screen
	// behind it — the platform default for a native Dialog. Registering on the root covers
	// every dialog in the app, Command.Dialog (the ⌘K palette) included, since it renders this.
	$effect(() => {
		if (!open) return;
		return onBackButton(() => (open = false));
	});
</script>

<DialogPrimitive.Root bind:open {...restProps} />
