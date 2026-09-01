<script lang="ts">
	let { open, onDismiss }: { open: boolean; onDismiss: () => void } = $props();
	let interactive = $state(false);

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
	class="fixed inset-0 z-[5] {interactive ? 'pointer-events-auto' : 'pointer-events-none'}"
	data-menu-dismiss-surface
	aria-hidden="true"
	onclick={onDismiss}
></div>
