<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { supporter, supporterFlow } from '$lib/stores/supporter.svelte';
	import SupporterBadge from '$lib/components/supporter-badge.svelte';
	import SupporterPerks from '$lib/components/supporter-perks.svelte';

	// Shown once a checkout completes (opened from $lib/paddle.ts). Closing it —
	// via Continue or any other way — resumes the action the paywall
	// interrupted, if there was one. `resume` nulls the pending action first,
	// so overlapping close paths (Continue sets the bound state AND may fire
	// onOpenChange) run it at most once.
	function resume() {
		const pending = supporterFlow.pending;
		supporterFlow.pending = null;
		if (pending && supporter.active) pending();
	}
	function handleOpenChange(open: boolean) {
		if (!open) resume();
	}
	function continueAndClose() {
		supporterFlow.thanksOpen = false;
		resume();
	}
</script>

<Dialog.Root bind:open={supporterFlow.thanksOpen} onOpenChange={handleOpenChange}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<SupporterBadge class="size-5" />
				Thank you — sincerely.
			</Dialog.Title>
			<Dialog.Description>
				Your support is what keeps Currents independent, ad-free, and getting better. It means a lot
				that you chose to back this project. Here's what's now yours:
			</Dialog.Description>
		</Dialog.Header>
		<SupporterPerks class="text-sm" />
		<p class="text-xs text-muted-foreground">
			You can manage your subscription anytime from Settings → Subscription.
		</p>
		<Button class="w-full" onclick={continueAndClose}>Continue</Button>
	</Dialog.Content>
</Dialog.Root>
