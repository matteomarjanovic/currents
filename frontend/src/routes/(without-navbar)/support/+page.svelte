<script lang="ts">
	import { resolve } from '$app/paths';
	import LogoMerged from '$lib/assets/logo.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import SiteFooter from '$lib/components/site-footer.svelte';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
</script>

<svelte:head>
	<title>Support · Currents</title>
	<meta
		name="description"
		content="Get help with Currents. Contact us about your account, saving images, subscriptions, refunds, or privacy."
	/>
</svelte:head>

<div class="min-h-svh px-4 py-8 md:px-8">
	<div class="mx-auto flex max-w-3xl flex-col gap-6">
		<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<a
				href={resolve('')}
				aria-label="Currents home"
				class="flex h-5 items-center gap-2 font-medium"
			>
				<LogoMerged />
			</a>
			<nav
				aria-label="Support navigation"
				class="flex flex-wrap items-center gap-3 text-sm text-muted-foreground"
			>
				<a class="underline underline-offset-4" href={resolve('terms')}>Terms</a>
				<a class="underline underline-offset-4" href={resolve('privacy')}>Privacy</a>
				<a class="underline underline-offset-4" href={resolve('refunds')}>Refunds</a>
				<a class="underline underline-offset-4" href={resolve('')}>Go to app</a>
				<ThemeToggle />
			</nav>
		</div>

		<Card.Root>
			<Card.Header class="gap-3">
				<Card.Title class="text-3xl leading-tight"><h1>Support</h1></Card.Title>
				<Card.Description>
					Need a hand with Currents? Get in touch using the contact that fits your question.
				</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-8 text-sm leading-6 text-foreground/90 md:text-[15px]">
				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">General help</h2>
					<p>
						For questions about your account, saving images, collections, imports, or the mobile
						apps, email
						<a class="underline underline-offset-4" href="mailto:support@currents.is"
							>support@currents.is</a
						>.
					</p>
					<p>
						If something isn't working, include your handle if relevant, your device and app or
						browser version, and the steps that led to the problem. Screenshots can help too. Please
						don't send passwords, sign-in tokens, or full payment card details.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">Subscriptions and refunds</h2>
					<p>
						For billing questions or refund requests, email
						<a class="underline underline-offset-4" href="mailto:refunds@currents.is"
							>refunds@currents.is</a
						>. Include the email address used at checkout and your order number if available. Read
						our <a class="underline underline-offset-4" href={resolve('refunds')}>Refund Policy</a>
						for eligibility and details.
					</p>
					<p>
						To manage or cancel a Supporter subscription, open
						<strong>Settings → Subscription → Manage</strong> in Currents to access your Polar billing
						portal.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">Privacy and account deletion</h2>
					<p>
						For privacy questions, personal data requests, or help deleting your account, email
						<a class="underline underline-offset-4" href="mailto:privacy@currents.is"
							>privacy@currents.is</a
						>. You can also start account deletion under <strong>Settings → Account</strong>. Our
						<a class="underline underline-offset-4" href={resolve('privacy')}>Privacy Policy</a>
						explains deletion options, retention, and how we handle your information.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">Bug reports and ideas</h2>
					<p>
						You can report a bug or suggest an improvement on
						<a
							class="underline underline-offset-4"
							href="https://userinput.app/s/did:plc:jaur46k6ijyfvl4lojza7eic/3mob6zfay3e2d"
							target="_blank"
							rel="noreferrer">our UserInput board</a
						>. Please use email for account details or other personal information.
					</p>
				</section>
			</Card.Content>
		</Card.Root>
	</div>
</div>

<SiteFooter />
