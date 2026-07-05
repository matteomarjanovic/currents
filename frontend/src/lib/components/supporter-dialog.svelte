<script lang="ts">
	import { resolve } from '$app/paths';
	import * as Dialog from '$lib/components/ui/dialog';
	import { supporter, supporterFlow } from '$lib/stores/supporter.svelte';
	import SupporterBadge from '$lib/components/supporter-badge.svelte';
	import SupporterPerks from '$lib/components/supporter-perks.svelte';
	import SupporterPlans from '$lib/components/supporter-plans.svelte';

	// Shown when a non-supporter reaches a supporter-tier feature (library
	// search, find similar in library). Subscribing opens the Polar embedded
	// checkout; the webhook-backed status poll unlocks the UI on completion.
	let { open = $bindable(false) }: { open?: boolean } = $props();

	// Close automatically if the entitlement flips while open (e.g. a checkout
	// completed in another tab).
	$effect(() => {
		if (open && supporter.active) open = false;
	});

	// If the paywall is dismissed without starting a checkout, the interrupted
	// action is stale — drop it so a later, unrelated checkout doesn't replay it.
	let startedCheckout = false;
	$effect(() => {
		if (open) startedCheckout = false;
	});
	function handleOpenChange(o: boolean) {
		if (!o && !startedCheckout) supporterFlow.pending = null;
	}
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<SupporterBadge class="size-5" />
				Support Currents
			</Dialog.Title>
			<Dialog.Description>
				Currents is an independent, ad-free project: it's funded by the people who use it, not by
				ads or your data. This feature is part of the supporter tier, which unlocks:
			</Dialog.Description>
		</Dialog.Header>
		<SupporterPerks class="text-sm" />
		<SupporterPlans
			onCheckoutOpen={() => {
				startedCheckout = true;
				open = false;
			}}
		/>
		<p class="text-xs text-muted-foreground">
			<a
				class="underline underline-offset-4 hover:text-foreground"
				href={resolve('/support-currents-project')}
				onclick={() => (open = false)}
			>
				Learn more about supporting the project</a
			>.
		</p>
	</Dialog.Content>
</Dialog.Root>
