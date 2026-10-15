<script lang="ts" module>
	// Shared form fields for single-image saves and each image in the page picker.
	// Alt text stays beside the image-specific form because it is always per-image.
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
	import SaveLabels from './SaveLabels.svelte';

	interface Props {
		details: Details;
		labels: SvelteSet<string>;
		disabled?: boolean;
		notePlaceholder?: string;
		labelsLabel?: string;
		showLabels?: boolean;
	}

	let {
		details,
		labels,
		disabled = false,
		notePlaceholder = 'Add a note (optional)',
		labelsLabel = 'Apply labels:',
		showLabels = true
	}: Props = $props();
</script>

<div class="flex flex-col gap-1">
	<span class="text-xs text-muted-foreground">Note</span>
	<Input type="text" placeholder={notePlaceholder} bind:value={details.note} {disabled} />
</div>

{#if showLabels}
	<SaveLabels {labels} {disabled} label={labelsLabel} />
{/if}

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
