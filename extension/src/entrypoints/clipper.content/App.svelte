<script lang="ts">
	import { clipper, hideClipper } from '../../lib/clipper-store.svelte';
	import LoginGate from './LoginGate.svelte';
	import SinglePicker from './SinglePicker.svelte';
	import MultiPicker from './MultiPicker.svelte';
	import { Button } from '$lib/components/ui/button';
	import X from '@lucide/svelte/icons/x';

	// A collection popover is open, so Escape belongs to it, not the dialog.
	let pickerOpen = $state(false);

	function close() {
		// A batch save runs inside the dialog; dismissing it would abandon the run.
		if (clipper.locked) return;
		hideClipper();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && !pickerOpen) close();
	}

	// Promote the backdrop into the browser top layer so page UI can't paint
	// over it, whatever z-index (or top layer) the page uses. Falls back to the
	// shadow host's max z-index where the Popover API is unavailable.
	function topLayer(node: HTMLElement) {
		try {
			node.showPopover?.();
		} catch {
			// already shown
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if clipper.visible}
	<div
		{@attach topLayer}
		popover="manual"
		class="fixed inset-0 isolate z-50 m-0 flex size-full items-center justify-center border-0 bg-black/30 p-0 font-sans"
		role="presentation"
		onclick={close}
	>
		<div
			class="relative flex max-h-[95vh] {clipper.mode === 'multi'
				? 'w-176'
				: 'w-90'} max-w-[calc(100vw-3rem)] flex-col gap-3 rounded-4xl bg-popover p-6 text-sm text-popover-foreground shadow-xl ring-1 ring-foreground/5 dark:ring-foreground/10"
			role="dialog"
			aria-modal="true"
			aria-label="Save to Currents"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
			onmousedown={(e) => e.stopPropagation()}
			onpointerdown={(e) => e.stopPropagation()}
		>
			<Button
				variant="ghost"
				size="icon-sm"
				class="absolute top-3 right-3 text-muted-foreground"
				onclick={close}
				disabled={clipper.locked}
				aria-label="Close"
			>
				<X />
			</Button>

			<!-- Keyed on the open, so reopening always starts from a clean form. -->
			{#key clipper.session}
				{#if clipper.authState === 'unauthenticated'}
					<LoginGate />
				{:else if clipper.mode === 'multi'}
					<MultiPicker onPickerOpenChange={(open) => (pickerOpen = open)} />
				{:else}
					<SinglePicker onPickerOpenChange={(open) => (pickerOpen = open)} />
				{/if}
			{/key}
		</div>
	</div>
{/if}
