<script lang="ts">
	import type { SvelteSet } from 'svelte/reactivity';

	interface Props {
		labels: SvelteSet<string>;
		disabled?: boolean;
		label?: string;
	}
	let { labels, disabled = false, label = 'Apply labels:' }: Props = $props();

	const options = [
		{ val: 'porn', label: 'Porn' },
		{ val: 'sexual', label: 'Sexual' },
		{ val: 'nudity', label: 'Nudity' },
		{ val: 'graphic-media', label: 'Graphic' },
		{ val: 'currents-ai-generated', label: 'AI-generated' }
	];
</script>

<div class="flex shrink-0 flex-col gap-1.5">
	<span class="text-xs text-muted-foreground">{label}</span>
	<div class="flex flex-wrap items-center gap-1.5 text-xs">
		{#each options as opt (opt.val)}
			{@const active = labels.has(opt.val)}
			<button
				type="button"
				aria-pressed={active}
				onclick={() => {
					if (!labels.delete(opt.val)) labels.add(opt.val);
				}}
				{disabled}
				class="rounded-full border px-2 py-0.5 transition-colors {active
					? 'border-foreground bg-foreground text-background'
					: 'border-border text-muted-foreground hover:bg-muted'}"
			>
				{opt.label}
			</button>
		{/each}
	</div>
</div>
