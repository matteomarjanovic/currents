<script lang="ts" module>
	// The metadata that applies to a save regardless of which image it is — so in
	// bulk mode one fill-in covers the whole selection. Alt text deliberately
	// isn't here: it describes one specific image.
	export interface Details {
		note: string;
		attributionCredit: string;
		attributionUrl: string;
		attributionLicense: string;
		showAttribution: boolean;
	}

	export function newDetails(credit = ''): Details {
		return {
			note: '',
			attributionCredit: credit,
			attributionUrl: '',
			attributionLicense: '',
			// Collapsed by default; expanded when the site pre-filled a credit so
			// the user sees what will be attributed.
			showAttribution: !!credit
		};
	}
</script>

<script lang="ts">
	import type { SvelteSet } from 'svelte/reactivity';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';

	interface Props {
		details: Details;
		labels: SvelteSet<string>;
		disabled?: boolean;
		notePlaceholder?: string;
		labelsLabel?: string;
	}

	let {
		details,
		labels,
		disabled = false,
		notePlaceholder = 'Add a note (optional)',
		labelsLabel = 'Apply labels:'
	}: Props = $props();

	const SELF_LABEL_OPTIONS: { val: string; label: string }[] = [
		{ val: 'porn', label: 'Porn' },
		{ val: 'sexual', label: 'Sexual' },
		{ val: 'nudity', label: 'Nudity' },
		{ val: 'graphic-media', label: 'Graphic' },
		{ val: 'currents-ai-generated', label: 'AI-generated' }
	];

	function toggleSelfLabel(val: string) {
		if (!labels.delete(val)) labels.add(val);
	}
</script>

<div class="flex flex-col gap-1">
	<span class="text-xs text-muted-foreground">Note</span>
	<Input type="text" placeholder={notePlaceholder} bind:value={details.note} {disabled} />
</div>

<span class="-mb-1 text-xs text-muted-foreground">{labelsLabel}</span>
<div class="flex flex-wrap items-center gap-1.5 text-xs">
	{#each SELF_LABEL_OPTIONS as opt (opt.val)}
		{@const active = labels.has(opt.val)}
		<button
			type="button"
			onclick={() => toggleSelfLabel(opt.val)}
			{disabled}
			class="rounded-full border px-2 py-0.5 transition-colors {active
				? 'border-foreground bg-foreground text-background'
				: 'border-border text-muted-foreground hover:bg-muted'}"
		>
			{opt.label}
		</button>
	{/each}
</div>

{#if details.showAttribution}
	<div class="flex flex-col gap-2">
		<span class="text-xs text-muted-foreground">Attribution</span>
		<Input
			type="text"
			placeholder="Credit (e.g. photographer name)"
			bind:value={details.attributionCredit}
			{disabled}
		/>
		<Input type="url" placeholder="Source URL" bind:value={details.attributionUrl} {disabled} />
		<Input
			type="text"
			placeholder="License (e.g. CC BY 4.0)"
			bind:value={details.attributionLicense}
			{disabled}
		/>
	</div>
{:else}
	<Button
		variant="link"
		size="sm"
		class="h-auto justify-start self-start p-0 text-foreground"
		onclick={() => (details.showAttribution = true)}
		{disabled}
	>
		+ Add attribution (recommended)
	</Button>
{/if}
