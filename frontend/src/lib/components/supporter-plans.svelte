<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { Button } from '$lib/components/ui/button';
	import { auth } from '$lib/stores/auth.svelte';
	import { isNative } from '$lib/platform';
	import {
		openSupporterCheckout,
		polarConfigured,
		POLAR_PRODUCT_MONTHLY,
		POLAR_PRODUCT_YEARLY
	} from '$lib/polar';

	// The two supporter price options, shared by the upgrade dialog and the
	// settings dialog's subscription section.
	let { onCheckoutOpen }: { onCheckoutOpen?: () => void } = $props();

	// App Store rules forbid selling digital goods in-app through an external
	// processor, so the native apps never show purchase buttons.
	const native = isNative();

	let canSubscribe = $derived(polarConfigured && !!auth.user);

	// The Polar embed renders above everything, so the host dialog closes
	// first (via onCheckoutOpen) — otherwise its focus trap fights the iframe.
	function subscribe(productId: string) {
		if (!auth.user) return;
		onCheckoutOpen?.();
		openSupporterCheckout(productId).catch(() => toast.error("Couldn't open the checkout"));
	}
</script>

{#if native}
	<p class="text-sm text-muted-foreground">Supporter subscriptions aren't available in the app.</p>
{:else}
	<div class="flex flex-col gap-2 sm:flex-row">
		<Button
			variant="outline"
			class="h-auto flex-1 flex-col gap-0.5 py-3"
			disabled={!canSubscribe}
			onclick={() => subscribe(POLAR_PRODUCT_MONTHLY)}
		>
			<span class="text-base font-semibold">$7 / month</span>
			<span class="text-xs font-normal text-muted-foreground">Billed monthly</span>
		</Button>
		<Button
			variant="outline"
			class="h-auto flex-1 flex-col gap-0.5 py-3"
			disabled={!canSubscribe}
			onclick={() => subscribe(POLAR_PRODUCT_YEARLY)}
		>
			<span class="text-base font-semibold">$70 / year</span>
			<span class="text-xs font-normal text-muted-foreground">2 months free</span>
		</Button>
	</div>
	<p class="text-xs text-muted-foreground">
		{#if polarConfigured}
			Payments are handled by Polar. Cancel anytime.
		{:else}
			Payments aren't configured in this environment yet.
		{/if}
	</p>
{/if}
