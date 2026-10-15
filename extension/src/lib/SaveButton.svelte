<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import Check from '@lucide/svelte/icons/check';

	let {
		state = 'idle',
		disabled = false,
		onclick
	}: {
		state?: 'idle' | 'saving' | 'saved';
		disabled?: boolean;
		onclick: () => void;
	} = $props();
	const label = $derived(
		state === 'saving' ? 'Saving…' : state === 'saved' ? 'Saved' : 'Save to Currents'
	);

	// Keep shared letters still; cross-blur and resize only the changing text.
	function morph(node: HTMLSpanElement, text: string) {
		let previous = text;
		let timer: ReturnType<typeof setTimeout>;
		node.textContent = text;
		return {
			update(next: string) {
				if (next === previous) return;
				clearTimeout(timer);
				let start = 0;
				while (start < previous.length && start < next.length && previous[start] === next[start])
					start++;
				let end = 0;
				while (
					end < previous.length - start &&
					end < next.length - start &&
					previous.at(-1 - end) === next.at(-1 - end)
				)
					end++;
				const swap = document.createElement('span');
				swap.className = 'label-swap';
				const outgoing = document.createElement('span');
				outgoing.textContent = previous.slice(start, previous.length - end);
				const incoming = document.createElement('span');
				incoming.textContent = next.slice(start, next.length - end);
				incoming.className = 'label-new';
				swap.append(outgoing, incoming);
				node.replaceChildren(next.slice(0, start), swap, next.slice(next.length - end));
				swap.style.width = `${outgoing.getBoundingClientRect().width}px`;
				void swap.offsetWidth;
				swap.classList.add('morphing');
				swap.style.width = `${incoming.getBoundingClientRect().width}px`;
				previous = next;
				timer = setTimeout(() => (node.textContent = next), 280);
			},
			destroy() {
				clearTimeout(timer);
			}
		};
	}
</script>

<Button
	{onclick}
	disabled={disabled || state !== 'idle'}
	aria-label={label}
	aria-busy={state === 'saving'}
	data-save-state={state}
	class="save-button w-full gap-0 {state !== 'idle' ? 'disabled:opacity-100' : ''} {state ===
	'saved'
		? 'bg-muted text-foreground'
		: ''}"
>
	<span class="icon-slot" class:expanded={state !== 'idle'} aria-hidden="true">
		<span class="icon" class:visible={state === 'saving'}><span class="spinner"></span></span>
		<span class="icon" class:visible={state === 'saved'}><Check class="size-4" /></span>
	</span>
	<span class="label" use:morph={label} aria-hidden="true"></span>
</Button>
<span class="sr-only" role="status">{state === 'idle' ? '' : label}</span>

<style>
	:global(.save-button) {
		transition:
			transform 150ms cubic-bezier(0.22, 1, 0.36, 1),
			background-color 250ms cubic-bezier(0.22, 1, 0.36, 1),
			color 250ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	:global(.save-button:active) {
		transform: scale(0.99);
	}
	.icon-slot {
		display: inline-grid;
		width: 0;
		margin-right: 0;
		overflow: hidden;
		transition:
			width 300ms cubic-bezier(0.22, 1, 0.36, 1),
			margin-right 300ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.icon-slot.expanded {
		width: 16px;
		margin-right: 6px;
	}
	.icon {
		grid-area: 1 / 1;
		display: grid;
		place-items: center;
		width: 16px;
		height: 16px;
		opacity: 0;
		filter: blur(2px);
		transform: scale(0.25);
		transition:
			opacity 250ms ease-in-out,
			filter 250ms ease-in-out,
			transform 250ms ease-in-out;
	}
	.icon.visible {
		opacity: 1;
		filter: blur(0);
		transform: scale(1);
	}
	.spinner {
		width: 14px;
		height: 14px;
		border-radius: 50%;
		background: conic-gradient(from 0deg, transparent 10%, currentColor);
		mask: radial-gradient(farthest-side, transparent calc(100% - 2px), #000 calc(100% - 1.5px));
		animation: spin 800ms linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	.label {
		white-space: nowrap;
	}
	.label :global(.label-swap) {
		display: inline-grid;
		vertical-align: bottom;
		justify-items: start;
		transition: width 240ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.label :global(.label-swap > span) {
		grid-area: 1 / 1;
		white-space: pre;
		transition:
			opacity 180ms cubic-bezier(0.22, 1, 0.36, 1),
			filter 180ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.label :global(.label-new),
	.label :global(.morphing > span:first-child) {
		opacity: 0;
		filter: blur(2px);
	}
	.label :global(.morphing > .label-new) {
		opacity: 1;
		filter: blur(0);
	}
	@media (prefers-reduced-motion: reduce) {
		:global(.save-button),
		.icon-slot,
		.icon,
		.label :global(.label-swap),
		.label :global(.label-swap > span) {
			transition: none;
		}
		:global(.save-button:active) {
			transform: none;
		}
		.spinner {
			animation-duration: 1600ms;
		}
	}
</style>
