<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Accordion from '$lib/components/ui/accordion/index.js';
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import { apiFetch } from '$lib/api';
	import { isNative } from '$lib/platform';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { supporter, loadSupporterStatus } from '$lib/stores/supporter.svelte';
	import {
		openSupporterCheckout,
		openSupporterPortal,
		polarConfigured,
		POLAR_PRODUCT_MONTHLY,
		POLAR_PRODUCT_YEARLY
	} from '$lib/polar';
	import { toast } from 'svelte-sonner';
	import SiteFooter from '$lib/components/site-footer.svelte';
	import SupporterBadge from '$lib/components/supporter-badge.svelte';
	import SupporterPerks from '$lib/components/supporter-perks.svelte';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import Heart from '@lucide/svelte/icons/heart';

	// The transparency numbers, fetched from the public stats endpoint. The
	// gross is estimated client-side because only the client knows which
	// product id is the monthly and which the yearly one.
	type SupporterStats = {
		totalUsers: number;
		supporters: number;
		byProduct: Record<string, number>;
	};
	let stats = $state<SupporterStats | null>(null);

	// auth.user is populated asynchronously by the layout, so react to it rather than
	// checking once in onMount (which could run before that fetch resolves).
	$effect(() => {
		if (auth.user && !supporter.loaded) void loadSupporterStatus();
	});

	onMount(async () => {
		try {
			const res = await apiFetch('/api/supporter/stats');
			if (res.ok) stats = (await res.json()) as SupporterStats;
		} catch {
			// the numbers simply stay as em dashes
		}
	});

	let monthlyGross = $derived.by(() => {
		if (!stats) return null;
		const monthly = stats.byProduct[POLAR_PRODUCT_MONTHLY] ?? 0;
		const yearly = stats.byProduct[POLAR_PRODUCT_YEARLY] ?? 0;
		return Math.round(monthly * 7 + (yearly * 70) / 12);
	});

	// App Store rules forbid selling digital goods in-app through an external
	// processor, so the native apps never show purchase buttons.
	const native = isNative();

	function subscribe(productId: string) {
		if (!auth.user) {
			promptLogin();
			return;
		}
		openSupporterCheckout(productId).catch(() => toast.error("Couldn't open the checkout"));
	}

	let portalLoading = $state(false);
	async function openPortal() {
		if (portalLoading) return;
		portalLoading = true;
		await openSupporterPortal();
		portalLoading = false;
	}
</script>

<svelte:head>
	<title>Become a supporter · Currents</title>
	<meta
		name="description"
		content="Support Currents: an independent, ad-free visual discovery app on the AT Protocol, funded by the people who use it."
	/>
</svelte:head>

<div class="mx-auto w-full max-w-2xl px-4 py-6 md:px-8 md:py-10">
	<article class="flex flex-col gap-10 text-[15px] leading-7 text-foreground/90">
		<section class="flex flex-col gap-4">
			<h1
				class="flex items-center gap-3 text-3xl leading-tight font-semibold tracking-tight text-foreground md:text-4xl"
			>
				<Heart class="size-7 shrink-0 fill-pink-500 text-pink-500 md:size-8" />
				Become a supporter
			</h1>
			<p>
				Currents was born from a simple need: a calm way to discover visual inspiration and manage a
				library of references, without the ads and the constant selling of your attention.
			</p>
			<p>
				That's not negotiable. Not selling your data is the hardest path, not the easiest. It's the
				reason the big alternatives are covered in ads, and it's exactly what we're trying to avoid.
			</p>
			<p>
				Which means this project depends entirely on people who find it useful and believe in it. It
				also means the only way it keeps existing is by staying worth supporting, there's no other
				funding behind it.
			</p>
		</section>

		<Card.Root class="shadow-none">
			<Card.Header class="gap-1.5">
				<Card.Title class="text-xl">Supporter</Card.Title>
				<Card.Description>One tier, everything included. Cancel anytime.</Card.Description>
			</Card.Header>
			<Card.Content>
				<SupporterPerks />
			</Card.Content>
			<Card.Footer class="flex-col items-stretch gap-3">
				{#if supporter.subscribed}
					<div
						class="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4"
					>
						<div class="flex flex-col gap-0.5">
							<span class="flex items-center gap-2 text-sm font-medium">
								<SupporterBadge class="size-5" />
								You're a supporter
							</span>
							<span class="text-xs text-muted-foreground">
								Thank you for keeping Currents running.
							</span>
						</div>
						<Button variant="outline" size="sm" disabled={portalLoading} onclick={openPortal}>
							Manage
							<ExternalLink class="size-3.5" />
						</Button>
					</div>
					<p class="text-xs text-muted-foreground">
						Invoices, payment method, plan changes, and cancellation are handled in the Polar
						billing portal.
					</p>
				{:else if native}
					<p class="text-sm text-muted-foreground">
						Supporter subscriptions aren't available in the app.
					</p>
				{:else}
					<div class="flex flex-col gap-2 sm:flex-row">
						<Button
							variant="default"
							class="h-auto flex-1 flex-col gap-0.5 py-3"
							disabled={!polarConfigured}
							onclick={() => subscribe(POLAR_PRODUCT_MONTHLY)}
						>
							<span class="text-base font-semibold">$7 / month</span>
							<span class="text-xs font-normal opacity-80">Billed monthly</span>
						</Button>
						<Button
							variant="outline"
							class="h-auto flex-1 flex-col gap-0.5 py-3"
							disabled={!polarConfigured}
							onclick={() => subscribe(POLAR_PRODUCT_YEARLY)}
						>
							<span class="text-base font-semibold">$70 / year</span>
							<span class="text-xs font-normal text-muted-foreground">2 months free</span>
						</Button>
					</div>
					<p class="text-xs text-muted-foreground">
						{#if polarConfigured}
							Payments are handled by Polar. Cancel anytime — see the
							<a class="underline underline-offset-4" href={resolve('/refunds')}>Refund Policy</a>.
						{:else}
							Supporter subscriptions are opening soon.
						{/if}
					</p>
				{/if}
			</Card.Footer>
		</Card.Root>

		<Separator />

		<section class="flex flex-col gap-4">
			<h2 class="text-xl font-semibold text-foreground">Transparency</h2>
			<p>
				Transparency is fundamental for us. We'll share what's happening behind the scenes,
				including how revenue gets used, as openly as we can.
			</p>

			<div class="grid grid-cols-3 gap-3 py-2">
				<div class="flex flex-col gap-1 rounded-lg border border-border bg-card p-4">
					<span class="text-2xl font-semibold tracking-tight text-foreground">
						{stats ? stats.totalUsers.toLocaleString() : '—'}
					</span>
					<span class="text-xs text-muted-foreground">People on Currents</span>
				</div>
				<div class="flex flex-col gap-1 rounded-lg border border-border bg-card p-4">
					<span class="text-2xl font-semibold tracking-tight text-foreground">
						{stats ? stats.supporters.toLocaleString() : '—'}
					</span>
					<span class="text-xs text-muted-foreground">Supporters</span>
				</div>
				<div class="flex flex-col gap-1 rounded-lg border border-border bg-card p-4">
					<span class="text-2xl font-semibold tracking-tight text-foreground">
						{monthlyGross === null ? '—' : `$${monthlyGross.toLocaleString()}`}
					</span>
					<span class="text-xs text-muted-foreground">Est. monthly gross</span>
				</div>
			</div>
		</section>

		<section class="flex flex-col gap-4">
			<h3 class="text-lg font-semibold text-foreground">What support goes towards</h3>
			<ol class="flex list-inside list-decimal flex-col gap-2">
				<li>Cloud migration (stable hosting, no more single point of failure)</li>
				<li>Paying the people who are working on the project</li>
			</ol>
			<p class="text-sm text-muted-foreground">
				Note: Right now it's just
				<a
					class="underline underline-offset-4"
					href="https://bsky.app/profile/matteomarjanovic.com"
					target="_blank"
					rel="noreferrer">me</a
				>. The moment support allows it, I want to bring in contractors to help with making this
				product better. That's what "paying the people working on the project" means.
			</p>
		</section>

		<section class="flex flex-col gap-4">
			<h3 class="text-lg font-semibold text-foreground">Roadmap</h3>
			<!-- Vertical stepper. Each marker column stacks a line segment above the
			     dot (h-2 puts the dot's center on the first text line of leading-7
			     text) and one below filling to the item's edge, so consecutive
			     segments touch and the line reads as continuous. The filled first
			     dot marks what's being worked on now. -->
			<ol class="flex flex-col">
				<li class="flex gap-4">
					<div class="flex w-3 flex-col items-center">
						<span class="h-2 w-px shrink-0"></span>
						<span class="size-3 shrink-0 rounded-full bg-foreground"></span>
						<span class="w-px flex-1 bg-border"></span>
					</div>
					<div class="pb-6"><em>Organize mode</em></div>
				</li>
				<li class="flex gap-4">
					<div class="flex w-3 flex-col items-center">
						<span class="h-2 w-px shrink-0 bg-border"></span>
						<span class="size-3 shrink-0 rounded-full bg-foreground"></span>
						<span class="w-px flex-1 bg-border"></span>
					</div>
					<div class="pb-6">Search by color</div>
				</li>
				<li class="flex gap-4">
					<div class="flex w-3 flex-col items-center">
						<span class="h-2 w-px shrink-0 bg-border"></span>
						<span class="size-3 shrink-0 rounded-full border-2 border-muted-foreground/50"></span>
						<span class="w-px flex-1 bg-border"></span>
					</div>
					<div class="pb-6">Mobile apps (Android and iOS)</div>
				</li>
				<li class="flex gap-4">
					<div class="flex w-3 flex-col items-center">
						<span class="h-2 w-px shrink-0 bg-border"></span>
						<span class="size-3 shrink-0 rounded-full border-2 border-muted-foreground/50"></span>
						<span class="w-px flex-1 bg-border"></span>
					</div>
					<div class="pb-6"><em>Following</em> feed</div>
				</li>
				<li class="flex gap-4">
					<div class="flex w-3 flex-col items-center">
						<span class="h-2 w-px shrink-0 bg-border"></span>
						<span class="size-3 shrink-0 rounded-full border-2 border-muted-foreground/50"></span>
					</div>
					<div>Much more to come...</div>
				</li>
			</ol>
			<p>
				For further insights, or for providing suggestions or feedback, visit
				<a class="underline underline-offset-4" href="https://currents.is/feedback"
					>currents.is/feedback</a
				>
			</p>
		</section>

		<section class="flex flex-col gap-4">
			<h2 class="text-xl font-semibold text-foreground">We're part of the Atmosphere</h2>
			<p>
				Currents is built on AT Protocol: a way of building social apps where you, not the platform,
				own your data.
			</p>
			<p>
				Today, if you get tired of how an app treats you, leaving means losing everything you built
				there. AT Protocol fixes that: your data lives with a provider you choose (Bluesky, Eurosky,
				Blacksky…), and you can move it, even to your own server, at any time. One account, portable
				across every app built this way.
			</p>
			<p>
				This growing ecosystem is called the Atmosphere. It's small, active, and full of people
				building the same kind of thing for the same reasons.
			</p>
			<p class="mt-2 -mb-2">Learn more:</p>
			<ul class="flex flex-col gap-2">
				<li>
					<a
						class="inline-flex items-center gap-1.5 underline underline-offset-4"
						href="https://www.youtube.com/watch?v=5YCBWuMoti0&t=12s&pp=ygUMIGRhbiBhYnJhbW92"
						target="_blank"
						rel="noreferrer"
					>
						youtube.com — About Atproto w/ Dan Abramov
						<ExternalLink class="size-3.5 shrink-0 text-muted-foreground" />
					</a>
				</li>
				<li>
					<a
						class="inline-flex items-center gap-1.5 underline underline-offset-4"
						href="https://atmosphereaccount.com/"
						target="_blank"
						rel="noreferrer"
					>
						atmosphereaccount.com
						<ExternalLink class="size-3.5 shrink-0 text-muted-foreground" />
					</a>
				</li>
				<li>
					<a
						class="inline-flex items-center gap-1.5 underline underline-offset-4"
						href="https://gui.do/post/atproto-series-01-you-dont-own-your-network/"
						target="_blank"
						rel="noreferrer"
					>
						gui.do — You don't own your network
						<ExternalLink class="size-3.5 shrink-0 text-muted-foreground" />
					</a>
				</li>
				<li>
					<a
						class="inline-flex items-center gap-1.5 underline underline-offset-4"
						href="https://atstore.fyi/"
						target="_blank"
						rel="noreferrer"
					>
						atstore.fyi
						<ExternalLink class="size-3.5 shrink-0 text-muted-foreground" />
					</a>
				</li>
			</ul>
		</section>

		<section class="flex flex-col gap-2">
			<h2 class="text-xl font-semibold text-foreground">FAQ</h2>
			<Accordion.Root type="single" class="w-full">
				<Accordion.Item value="cancel">
					<Accordion.Trigger>Can I cancel anytime?</Accordion.Trigger>
					<Accordion.Content>
						Yes. Open Settings → Subscription → Manage and cancel in a couple of clicks. You keep
						supporter features until the end of the period you've paid for. And if you change your
						mind right after subscribing, your first payment is fully refundable within 14 days (see
						the <a class="underline underline-offset-4" href={resolve('/refunds')}>Refund Policy</a
						>).
					</Accordion.Content>
				</Accordion.Item>
				<Accordion.Item value="charity">
					<Accordion.Trigger>Is this charity?</Accordion.Trigger>
					<Accordion.Content>
						No, it's a subscription to a product, with receipts and invoices like any other service.
						Think of it as backing an independent app you actually use: your money pays for the
						service and keeps it ad-free and independent, instead of your data paying for it.
					</Accordion.Content>
				</Accordion.Item>
				<Accordion.Item value="shutdown">
					<Accordion.Trigger>What happens to my data if Currents shuts down?</Accordion.Trigger>
					<Accordion.Content>
						It stays yours. Your collections and saves live on your AT Protocol account (your PDS),
						not on Currents' servers, so they don't disappear with us, and any other app built on
						the protocol can read them. The Currents code is open source too, so the service itself
						could be run by someone else.
					</Accordion.Content>
				</Accordion.Item>
				<Accordion.Item value="costs">
					<Accordion.Trigger>What does the money pay for?</Accordion.Trigger>
					<Accordion.Content>
						Servers, the database, the GPU inference that powers semantic and visual search, and the
						development time to keep improving Currents.
					</Accordion.Content>
				</Accordion.Item>
				<Accordion.Item value="payment-safety">
					<Accordion.Trigger>Is my payment information safe?</Accordion.Trigger>
					<Accordion.Content>
						Payments are processed by Polar, our merchant of record. Your card details go to Polar
						and never touch Currents' servers.
					</Accordion.Content>
				</Accordion.Item>
				<Accordion.Item value="free-support">
					<Accordion.Trigger>Can I support the project without paying?</Accordion.Trigger>
					<Accordion.Content>
						Absolutely: use the app, tell us what's broken or missing at
						<a class="underline underline-offset-4" href="https://currents.is/feedback"
							>currents.is/feedback</a
						>, share Currents with someone who'd like it, or star the project on GitHub.
					</Accordion.Content>
				</Accordion.Item>
			</Accordion.Root>
		</section>
	</article>
</div>

<SiteFooter class="mt-10" />
