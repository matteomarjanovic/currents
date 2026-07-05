<script lang="ts">
	import { resolve } from '$app/paths';
	import LogoMerged from '$lib/assets/logo.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import SiteFooter from '$lib/components/site-footer.svelte';
</script>

<svelte:head>
	<title>Refund Policy · Currents</title>
	<meta
		name="description"
		content="Refund Policy for the Currents Supporter subscription, sold through Polar as merchant of record."
	/>
</svelte:head>

<div class="min-h-svh px-4 py-8 md:px-8">
	<div class="mx-auto flex max-w-3xl flex-col gap-6">
		<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<a href={resolve('/')} class="flex h-5 items-center gap-2 font-medium">
				<LogoMerged />
			</a>
			<div class="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
				<a class="underline underline-offset-4" href={resolve('/terms')}>Terms</a>
				<a class="underline underline-offset-4" href={resolve('/privacy')}>Privacy</a>
				<a class="underline underline-offset-4" href={resolve('/refunds')}>Refunds</a>
				<a class="underline underline-offset-4" href={resolve('/login')}>Log in</a>
			</div>
		</div>

		<Card.Root>
			<Card.Header class="gap-3">
				<Card.Title class="text-3xl leading-tight">Refund Policy</Card.Title>
				<Card.Description>
					Last updated July 2, 2026. This policy covers payments for the Currents Supporter
					subscription.
				</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-8 text-sm leading-6 text-foreground/90 md:text-[15px]">
				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">1. What this policy covers</h2>
					<p>
						Currents offers an optional paid tier called <strong>Supporter</strong> ($7 per month or
						$70 per year) that unlocks additional features such as semantic text search and visual
						similar-image search within your library. Payments are processed by
						<a
							class="underline underline-offset-4"
							href="https://polar.sh"
							target="_blank"
							rel="noreferrer">Polar</a
						>, our merchant of record — the charge on your statement appears from Polar, and refunds
						are returned by Polar to your original payment method.
					</p>
					<p>The free saving and collecting experience involves no payments to refund.</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">2. 14-day money-back guarantee</h2>
					<p>
						If you are not happy with your Supporter subscription, you can request a
						<strong>full refund of your first payment within 14 days</strong> of the initial purchase
						— no questions asked.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">3. Renewal payments</h2>
					<p>
						Subscriptions renew automatically at the end of each billing period. A renewal charge
						can be refunded if you contact us within <strong>14 days</strong> of the charge — for
						example, if you meant to cancel and forgot. To avoid unwanted renewals, you can cancel
						anytime from <em>Settings → Subscription → Manage</em>; you keep Supporter features
						until the end of the paid period. Cancelling by itself does not issue a refund.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">4. How to request a refund</h2>
					<p>
						Email
						<a class="underline underline-offset-4" href="mailto:refunds@currents.is"
							>refunds@currents.is</a
						>
						from the email address you used at checkout, ideally including the order number from your
						Polar receipt. You can also reply directly to the Polar receipt email.
					</p>
					<p>
						Approved refunds are processed by Polar back to your original payment method, typically
						within 5–10 business days depending on your bank. When a refund is issued, the
						subscription ends and Supporter features are disabled.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">5. Your statutory rights</h2>
					<p>
						Nothing in this policy limits any non-waivable rights you have under the
						consumer-protection laws of your country, including any statutory withdrawal or
						cancellation rights.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">6. Contact</h2>
					<p>
						For refund requests, email
						<a class="underline underline-offset-4" href="mailto:refunds@currents.is"
							>refunds@currents.is</a
						>. For any other questions about the service, email
						<a class="underline underline-offset-4" href="mailto:support@currents.is"
							>support@currents.is</a
						>.
					</p>
				</section>
			</Card.Content>
		</Card.Root>
	</div>
</div>

<SiteFooter />
