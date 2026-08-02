<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { Button } from '$lib/components/ui/button';
	import { auth } from '$lib/stores/auth.svelte';
	import { isNative, isAndroid } from '$lib/platform';
	import { openExternal } from '$lib/external';
	import {
		openSupporterCheckout,
		polarConfigured,
		POLAR_PRODUCT_MONTHLY,
		POLAR_PRODUCT_YEARLY
	} from '$lib/polar';

	// The two supporter price options, shared by the upgrade dialog and the
	// settings dialog's subscription section.
	let { onCheckoutOpen }: { onCheckoutOpen?: () => void } = $props();

	// App Store rules forbid selling digital goods in-app through an external processor, so the
	// native apps never run the in-app Polar embed. Google Play permits linking out, so on Android
	// we hand off to the web checkout in the system browser; iOS stays hidden.
	const native = isNative();
	const android = isAndroid();

	let canSubscribe = $derived(polarConfigured && !!auth.user);

	// The Polar embed renders above everything, so the host dialog closes
	// first (via onCheckoutOpen) — otherwise its focus trap fights the iframe.
	function subscribe(productId: string) {
		if (!auth.user) return;
		onCheckoutOpen?.();
		openSupporterCheckout(productId).catch(() => toast.error("Couldn't open the checkout"));
	}

	// Android hands off to the web checkout: close the host dialog, then open /support-us in the
	// system browser (where isNative() is false, so it isn't redirected away).
	function openWebCheckout() {
		onCheckoutOpen?.();
		openExternal('/support-us');
	}
</script>

{#if android}
	<Button variant="outline" class="w-full" onclick={openWebCheckout}>Become a supporter</Button>
	<p class="text-xs text-muted-foreground">Opens in your browser to complete checkout.</p>
{:else if native}
	<div class="flex flex-col gap-1.5 text-sm text-muted-foreground">
		<p>Apple doesn't allow paid subscriptions to be sold inside the app.</p>
		<p>
			To become a supporter, open <span class="font-medium text-foreground">currents.is</span> in your
			browser, sign in, and subscribe from there — your account unlocks automatically.
		</p>
	</div>
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
