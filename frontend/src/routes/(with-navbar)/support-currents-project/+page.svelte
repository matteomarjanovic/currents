<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Accordion from '$lib/components/ui/accordion/index.js';
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { supporter, loadSupporterStatus } from '$lib/stores/supporter.svelte';
	import {
		openSupporterCheckout,
		paddleConfigured,
		PADDLE_PRICE_MONTHLY,
		PADDLE_PRICE_YEARLY
	} from '$lib/paddle';
	import SiteFooter from '$lib/components/site-footer.svelte';
	import SupporterBadge from '$lib/components/supporter-badge.svelte';
	import SupporterPerks from '$lib/components/supporter-perks.svelte';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import Heart from '@lucide/svelte/icons/heart';

	// The transparency numbers, fetched from the public stats endpoint. The
	// gross is estimated client-side because only the client knows which price
	// id is the monthly and which the yearly one.
	type SupporterStats = { totalUsers: number; supporters: number; byPrice: Record<string, number> };
	let stats = $state<SupporterStats | null>(null);

	onMount(async () => {
		if (auth.user && !supporter.loaded) void loadSupporterStatus();
		try {
			const res = await apiFetch('/api/supporter/stats');
			if (res.ok) stats = (await res.json()) as SupporterStats;
		} catch {
			// the numbers simply stay as em dashes
		}
	});

	let monthlyGross = $derived.by(() => {
		if (!stats) return null;
		const monthly = stats.byPrice[PADDLE_PRICE_MONTHLY] ?? 0;
		const yearly = stats.byPrice[PADDLE_PRICE_YEARLY] ?? 0;
		return Math.round(monthly * 7 + (yearly * 70) / 12);
	});

	function subscribe(priceId: string) {
		if (!auth.user) {
			promptLogin();
			return;
		}
		void openSupporterCheckout(priceId, auth.user.did);
	}
</script>

<svelte:head>
	<title>Become a supporter · Currents</title>
	<meta
		name="description"
		content="Support Currents — an independent, ad-free visual discovery app on the AT Protocol, funded by the people who use it."
	/>
</svelte:head>

<div class="mx-auto w-full max-w-2xl px-2 py-6 md:py-10">
	<article class="flex flex-col gap-10 text-[15px] leading-7 text-foreground/90">
		<section class="flex flex-col gap-4">
			<h1
				class="flex items-center gap-3 text-3xl leading-tight font-semibold tracking-tight text-foreground md:text-4xl"
			>
				<Heart class="size-7 shrink-0 fill-pink-500 text-pink-500 md:size-8" />
				Become a supporter
			</h1>
			<p>
				Currents is a project that was born from a real need: an app that lets users discover new
				visual inspiration and manage their library of references in a seamless way.
			</p>
			<p>
				One aspect is not negotiable: it shouldn't sell users data. That would be the easy path for
				financial success, that's why the main alternatives pack their UIs with ads. But the web
				doesn't have to be that way: it can be of the <i>people</i>.
			</p>
			<p>
				This is why this project depends entirely on the people who find it useful, believe in its
				values and decide to support it financially. And this means also that it's our
				responsibility to provide the best product we can, since that's the single thing our users
				want and would support us for.
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
					<p class="flex items-center gap-2 text-sm font-medium">
						<SupporterBadge class="size-5" />
						You're already a supporter — thank you.
					</p>
					<p class="text-xs text-muted-foreground">
						You can manage your subscription anytime from Settings → Subscription.
					</p>
				{:else}
					<div class="flex flex-col gap-2 sm:flex-row">
						<Button
							variant="default"
							class="h-auto flex-1 flex-col gap-0.5 py-3"
							disabled={!paddleConfigured}
							onclick={() => subscribe(PADDLE_PRICE_MONTHLY)}
						>
							<span class="text-base font-semibold">$7 / month</span>
							<span class="text-xs font-normal opacity-80">Billed monthly</span>
						</Button>
						<Button
							variant="outline"
							class="h-auto flex-1 flex-col gap-0.5 py-3"
							disabled={!paddleConfigured}
							onclick={() => subscribe(PADDLE_PRICE_YEARLY)}
						>
							<span class="text-base font-semibold">$70 / year</span>
							<span class="text-xs font-normal text-muted-foreground">2 months free</span>
						</Button>
					</div>
					<p class="text-xs text-muted-foreground">
						{#if paddleConfigured}
							Payments are handled by Paddle. Cancel anytime — see the
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
				One of the values we strongly believe in is transparency. That's why we'll do our best to
				communicate with our community all the things that are happening behind the scenes, as our
				estimated revenue allocation.
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

			<p>
				The first supporters will help covering the infrastructure costs we already have. This will
				ensure higher uptime and reliability of the service.
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
					<div class="pb-6">Improve the <em>Organize mode</em></div>
				</li>
				<li class="flex gap-4">
					<div class="flex w-3 flex-col items-center">
						<span class="h-2 w-px shrink-0 bg-border"></span>
						<span class="size-3 shrink-0 rounded-full border-2 border-muted-foreground/50"></span>
						<span class="w-px flex-1 bg-border"></span>
					</div>
					<div class="pb-6">Release mobile apps (Android and iOS)</div>
				</li>
				<li class="flex gap-4">
					<div class="flex w-3 flex-col items-center">
						<span class="h-2 w-px shrink-0 bg-border"></span>
						<span class="size-3 shrink-0 rounded-full border-2 border-muted-foreground/50"></span>
						<span class="w-px flex-1 bg-border"></span>
					</div>
					<div class="pb-6">Create a <em>Following</em> feed</div>
				</li>
				<li class="flex gap-4">
					<div class="flex w-3 flex-col items-center">
						<span class="h-2 w-px shrink-0 bg-border"></span>
						<span class="size-3 shrink-0 rounded-full border-2 border-muted-foreground/50"></span>
						<span class="w-px flex-1 bg-border"></span>
					</div>
					<div class="pb-6">Search by color</div>
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
					>https://currents.is/feedback</a
				>
			</p>
		</section>

		<section class="flex flex-col gap-4">
			<h2 class="text-xl font-semibold text-foreground">We're part of the Atmosphere</h2>
			<p>
				Currents is built on a technology called AT Protocol. Don't be scared by the techy-sounding
				name, it's just a new way to creating social apps that are open and where you're the owner
				of your data.
			</p>
			<p>
				Common apps store your data on their server, and make migrating to another service almost
				impossible: if you have used their app for a long time but you don't like anymore how they
				handle it, you're basically locked-in with them. With this new approach, you choose a
				provider (like Bluesky, Eurosky, Blacksky) that will store your data on their servers. And
				the nice thing is that you can very easily migrate across them; you could even move your
				data on a server at your place!
			</p>
			<p>
				With this new paradigm, the data of all your social apps that use this technology are in the
				same place: you only need one account to have access to all of them!
			</p>
			<p>
				The ecosystem of these new wave of open social apps is called Atmosphere, and it's very
				active and full of welcoming and enthusiast people. To find out more about this, you can
				check out these pages:
			</p>
			<ul class="flex flex-col gap-2">
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
						Yes. Open Settings → Subscription → Manage and cancel in a couple of clicks — you keep
						supporter features until the end of the period you've paid for. And if you change your
						mind right after subscribing, your first payment is fully refundable within 14 days (see
						the <a class="underline underline-offset-4" href={resolve('/refunds')}>Refund Policy</a
						>).
					</Accordion.Content>
				</Accordion.Item>
				<Accordion.Item value="charity">
					<Accordion.Trigger>Is this a charity?</Accordion.Trigger>
					<Accordion.Content>
						No — it's a subscription to a product, with receipts and invoices like any other
						service. Think of it as backing an independent app you actually use: your money pays for
						the service and keeps it ad-free and independent, instead of your data paying for it.
					</Accordion.Content>
				</Accordion.Item>
				<Accordion.Item value="shutdown">
					<Accordion.Trigger>What happens to my data if Currents shuts down?</Accordion.Trigger>
					<Accordion.Content>
						It stays yours. Your collections and saves live on your AT Protocol account (your PDS),
						not on Currents' servers — so they don't disappear with us, and any other app built on
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
						Payments are processed by Paddle, our merchant of record — your card details go to
						Paddle and never touch Currents' servers.
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

<!-- Negative margins cancel the with-navbar layout's main padding so the
     footer runs edge to edge, like on the landing page. -->
<SiteFooter class="-mx-2 mt-10 -mb-2 md:-mx-4 md:-mb-4" />
