<script lang="ts">
	import { AlertDialog as AlertDialogPrimitive } from "bits-ui";
	import { onBackButton } from "$lib/back-button";

	let { open = $bindable(false), ...restProps }: AlertDialogPrimitive.RootProps = $props();

	// Same as dialog.svelte: back dismisses rather than navigating away underneath.
	// Cancelling is the safe reading of back on a confirm prompt.
	$effect(() => {
		if (!open) return;
		return onBackButton(() => (open = false));
	});
</script>

<AlertDialogPrimitive.Root bind:open {...restProps} />
