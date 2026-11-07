<script lang="ts">
	import { cn } from '$lib/utils';

	let {
		open,
		onDismiss,
		class: className
	}: { open: boolean; onDismiss: () => void; class?: string } = $props();
	let interactive = $state(false);
	let pressed = false;

	$effect(() => {
		if (!open) {
			interactive = false;
			return;
		}
		const frame = requestAnimationFrame(() => (interactive = true));
		return () => cancelAnimationFrame(frame);
	});
</script>

<div
	class={cn(
		'fixed inset-0 z-[5]',
		interactive ? 'pointer-events-auto' : 'pointer-events-none',
		className
	)}
	data-menu-dismiss-surface
	aria-hidden="true"
	onpointerdown={() => (pressed = true)}
	onpointercancel={() => (pressed = false)}
	onclick={(event) => {
		event.preventDefault();
		event.stopPropagation();
		// A long-press can mount this layer beneath an already-held touch.
		// Only dismiss a gesture that actually started on the layer.
		if (!pressed) return;
		pressed = false;
		onDismiss();
	}}
></div>
