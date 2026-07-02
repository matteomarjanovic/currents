<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import LogoMerged from '$lib/assets/logo.svelte';
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
	import Check from '@lucide/svelte/icons/check';
	import ExternalLink from '@lucide/svelte/icons/external-link';

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

	const PERKS = [
		'Semantic search in your library',
		'Image search in your library ("find similar")',
		'Both filterable by collection',
		'A supporter badge',
		'Much more to come'
	];

	function subscribe(priceId: string) {
		if (!auth.user) {
			promptLogin();
			return;
		}
		void openSupporterCheckout(priceId, auth.user.did);
	}
</script>

<svelte:head>
	<title>Become a Supporter · Currents</title>
	<meta
		name="description"
		content="Support Currents — an independent, ad-free visual discovery app on the AT Protocol, funded by the people who use it."
	/>
</svelte:head>

<div class="min-h-svh px-4 py-8 md:px-8">
	<div class="mx-auto flex max-w-2xl flex-col gap-10">
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

		<article class="flex flex-col gap-10 text-[15px] leading-7 text-foreground/90">
			<section class="flex flex-col gap-4">
				<h1 class="text-3xl leading-tight font-semibold tracking-tight text-foreground md:text-4xl">
					Become a Currents Supporter
				</h1>
				<p>
					Currents is a project that was born from a real need: an app that lets users discover new
					visual inspiration and manage their library of references in a seamless way.
				</p>
				<p>
					One aspect is not negotiable: this shouldn't sell users data. That would be the easy path
					for financial success, that's why the main alternatives pack their UIs with ads. But the
					web doesn't have to be that way: it can be of the people.
				</p>
				<p>
					This means that this project depends entirely on the people who find it useful, believe in
					its values and decide to support it financially. And this means also that it's our
					responsibility to provide the best product we can, since that's the single thing our users
					want and would support us for.
				</p>
			</section>

			<Card.Root>
				<Card.Header class="gap-1.5">
					<Card.Title class="text-xl">Supporter</Card.Title>
					<Card.Description>One tier, everything included. Cancel anytime.</Card.Description>
				</Card.Header>
				<Card.Content>
					<ul class="flex flex-col gap-2.5">
						{#each PERKS as perk (perk)}
							<li class="flex items-start gap-2.5">
								<Check class="mt-1 size-4 shrink-0 text-foreground" />
								<span>{perk}</span>
							</li>
						{/each}
					</ul>
				</Card.Content>
				<Card.Footer class="flex-col items-stretch gap-3">
					{#if supporter.subscribed}
						<p class="text-sm font-medium">You're already a supporter — thank you.</p>
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
								<a class="underline underline-offset-4" href={resolve('/refunds')}>Refund Policy</a
								>.
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
					The first supporters will help covering the infrastructure costs we already have. This
					will ensure higher uptime and reliability of the service.
				</p>
			</section>

			<section class="flex flex-col gap-4">
				<h3 class="text-lg font-semibold text-foreground">Roadmap</h3>
				<ul class="list-disc space-y-2 pl-5 marker:text-muted-foreground">
					<li>Improve the <em>Organize mode</em></li>
					<li>Release mobile apps (Android and iOS)</li>
					<li>Create a <em>Following</em> feed</li>
				</ul>
				<p>
					... and much more! For further insights, or for providing suggestions or feedback, visit
					<a class="underline underline-offset-4" href="https://currents.is/feedback"
						>https://currents.is/feedback</a
					>
				</p>
			</section>

			<section class="flex flex-col gap-4">
				<h2 class="text-xl font-semibold text-foreground">We're part of the Atmosphere</h2>
				<p>
					Currents is built on a technology called AT Protocol. Don't be scared by the
					techy-sounding name, it's just a new way to creating social apps that are open and where
					you're the owner of your data.
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
					With this new paradigm, the data of all your social apps that use this technology are in
					the same place: you only need one account to have access to all of them!
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
							mind right after subscribing, your first payment is fully refundable within 14 days
							(see the <a class="underline underline-offset-4" href={resolve('/refunds')}
								>Refund Policy</a
							>).
						</Accordion.Content>
					</Accordion.Item>
					<Accordion.Item value="charity">
						<Accordion.Trigger>Is this a charity?</Accordion.Trigger>
						<Accordion.Content>
							No — it's a subscription to a product, with receipts and invoices like any other
							service. Think of it as backing an independent app you actually use: your money pays
							for the service and keeps it ad-free and independent, instead of your data paying for
							it.
						</Accordion.Content>
					</Accordion.Item>
					<Accordion.Item value="shutdown">
						<Accordion.Trigger>What happens to my data if Currents shuts down?</Accordion.Trigger>
						<Accordion.Content>
							It stays yours. Your collections and saves live on your AT Protocol account (your
							PDS), not on Currents' servers — so they don't disappear with us, and any other app
							built on the protocol can read them. The Currents code is open source too, so the
							service itself could be run by someone else.
						</Accordion.Content>
					</Accordion.Item>
					<Accordion.Item value="costs">
						<Accordion.Trigger>What does the money pay for?</Accordion.Trigger>
						<Accordion.Content>
							Servers, the database, the GPU inference that powers semantic and visual search, and
							the development time to keep improving Currents.
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
</div>
