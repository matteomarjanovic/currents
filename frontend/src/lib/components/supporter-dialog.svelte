<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { supporter } from '$lib/stores/supporter.svelte';
	import SupporterPlans from '$lib/components/supporter-plans.svelte';
	import Sparkles from '@lucide/svelte/icons/sparkles';

	// Shown when a non-supporter reaches a supporter-tier feature (library
	// search, find similar in library). Subscribing opens the Paddle overlay
	// checkout; the webhook-backed status poll unlocks the UI on completion.
	let { open = $bindable(false) }: { open?: boolean } = $props();

	// Close automatically if the entitlement flips while open (e.g. a checkout
	// completed in another tab).
	$effect(() => {
		if (open && supporter.active) open = false;
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<Sparkles class="size-4" />
				Support Currents
			</Dialog.Title>
			<Dialog.Description>
				Searching your library by meaning and finding visually similar images run on Currents' AI
				infrastructure. They're reserved for supporters — a subscription that keeps Currents running
				and independent.
			</Dialog.Description>
		</Dialog.Header>
		<SupporterPlans onCheckoutOpen={() => (open = false)} />
	</Dialog.Content>
</Dialog.Root>
