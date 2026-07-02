<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		openSupporterCheckout,
		paddleConfigured,
		PADDLE_PRICE_MONTHLY,
		PADDLE_PRICE_YEARLY
	} from '$lib/paddle';

	// The two supporter price options, shared by the upgrade dialog and the
	// settings dialog's subscription section.
	let { onCheckoutOpen }: { onCheckoutOpen?: () => void } = $props();

	let canSubscribe = $derived(paddleConfigured && !!auth.user);

	// The Paddle overlay renders above everything, so the host dialog closes
	// first (via onCheckoutOpen) — otherwise its focus trap fights the iframe.
	function subscribe(priceId: string) {
		const did = auth.user?.did;
		if (!did) return;
		onCheckoutOpen?.();
		void openSupporterCheckout(priceId, did);
	}
</script>

<div class="flex flex-col gap-2 sm:flex-row">
	<Button
		variant="outline"
		class="h-auto flex-1 flex-col gap-0.5 py-3"
		disabled={!canSubscribe}
		onclick={() => subscribe(PADDLE_PRICE_MONTHLY)}
	>
		<span class="text-base font-semibold">$7 / month</span>
		<span class="text-xs font-normal text-muted-foreground">Billed monthly</span>
	</Button>
	<Button
		variant="outline"
		class="h-auto flex-1 flex-col gap-0.5 py-3"
		disabled={!canSubscribe}
		onclick={() => subscribe(PADDLE_PRICE_YEARLY)}
	>
		<span class="text-base font-semibold">$70 / year</span>
		<span class="text-xs font-normal text-muted-foreground">2 months free</span>
	</Button>
</div>
<p class="text-xs text-muted-foreground">
	{#if paddleConfigured}
		Payments are handled by Paddle. Cancel anytime.
	{:else}
		Payments aren't configured in this environment yet.
	{/if}
</p>
