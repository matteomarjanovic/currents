<script lang="ts">
	import { resolve } from '$app/paths';
	import LogoMerged from '$lib/assets/logo.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
</script>

<svelte:head>
	<title>Terms of Service • Currents</title>
	<meta
		name="description"
		content="Terms of Service for the hosted Currents web app, browser extension, APIs, and related services."
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
				<a class="underline underline-offset-4" href={resolve('/login')}>Log in</a>
			</div>
		</div>

		<Card.Root>
			<Card.Header class="gap-3">
				<Card.Title class="text-3xl leading-tight">Terms of Service</Card.Title>
				<Card.Description>
					Last updated April 22, 2026. These Terms apply to the hosted Currents service, including
					the web app, browser extension, APIs, appview, and related infrastructure.
				</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-8 text-sm leading-6 text-foreground/90 md:text-[15px]">
				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">1. Scope</h2>
					<p>
						Currents is a visual bookmarking service built on the AT Protocol. These Terms govern
						your use of the hosted Currents experience that we operate. They do not govern
						third-party Personal Data Servers, other AT Protocol clients, or self-hosted forks of
						the open-source codebase.
					</p>
					<p>If you do not agree to these Terms, do not use the hosted Currents service.</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">2. Eligibility and accounts</h2>
					<p>
						You must be at least 13 years old, or the minimum age required to consent to data
						processing where you live, to use Currents. If you are under the age of majority in your
						jurisdiction, you may use Currents only with permission from a parent or legal guardian.
					</p>
					<p>
						Currents uses an Atmosphere or other compatible AT Protocol account for login. You are
						responsible for your account, your PDS relationship, and activity carried out through
						your session.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">3. Public networked service</h2>
					<p>
						Currents is built around public AT Protocol records. Collections, saves, profile
						information, and related metadata you publish through Currents may be visible to other
						users and may also be available through the broader AT Protocol ecosystem.
					</p>
					<p>
						The hosted Currents service indexes public records into its own search and discovery
						infrastructure. Removing something from one interface does not instantly remove it from
						other caches, mirrors, protocol peers, or third-party services.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">4. Your content</h2>
					<p>
						You retain ownership of the content you upload or publish. You are solely responsible
						for making sure you have the rights, permissions, and legal basis needed to post it,
						including any image, text, link, attribution, or credit information attached to a save.
					</p>
					<p>Do not use Currents to publish or distribute content that:</p>
					<ul class="list-disc space-y-2 pl-5 marker:text-muted-foreground">
						<li>Infringes another person's intellectual property or privacy rights.</li>
						<li>Is unlawful, fraudulent, deceptive, or abusive.</li>
						<li>Contains malware or is meant to interfere with the service or other users.</li>
						<li>Includes sensitive or private material you do not want to become public.</li>
					</ul>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">5. License you grant to Currents</h2>
					<p>
						To operate the hosted service, you grant us a non-exclusive, worldwide, royalty-free
						license to host, copy, cache, index, display, reproduce, adapt, and transmit your
						content as needed to run Currents. This includes generating thumbnails, embeddings,
						dominant-color palettes, dimensions, related-save results, and other technical
						derivatives used for search, recommendations, and ranking.
					</p>
					<p>
						This license lasts only as long as needed to operate the hosted service and honor
						reasonable backups, caches, and audit trails. It does not transfer ownership of your
						content to us.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">6. Acceptable use</h2>
					<p>You agree not to:</p>
					<ul class="list-disc space-y-2 pl-5 marker:text-muted-foreground">
						<li>Scrape or harvest Currents data in unauthorized or abusive ways.</li>
						<li>Attempt to bypass security, authentication, or rate limits.</li>
						<li>Impersonate another person or misrepresent your affiliation.</li>
						<li>Use Currents to spam, phish, harass, or stalk other people.</li>
						<li>
							Interfere with the AT Protocol network, third-party PDS infrastructure, or Currents'
							hosted systems.
						</li>
					</ul>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">
						7. Search, recommendations, and ranking
					</h2>
					<p>
						Currents uses machine-learning systems to support semantic search, related-save
						retrieval, and discovery feeds. That means your public saves may be analyzed to create
						vectors and other derived metadata, and your search terms may be processed to find
						relevant results.
					</p>
					<p>
						These systems are used to run the product. They do not guarantee any specific outcome,
						visibility level, or ranking position.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">8. Third-party services and content</h2>
					<p>
						Currents depends on third-party services, including AT Protocol infrastructure, identity
						providers, PDS operators, and linked websites. We are not responsible for the
						availability, policies, or content of those third parties.
					</p>
					<p>If you save or visit an external link through Currents, you do so at your own risk.</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">
						9. Suspension, termination, and changes
					</h2>
					<p>
						We may suspend, restrict, or terminate access to the hosted service if you violate these
						Terms, create risk for other users or the network, or if we need to protect the service,
						comply with the law, or shut down or materially change a feature.
					</p>
					<p>
						You can stop using Currents at any time. Because your content may also live on your PDS
						or elsewhere on the AT Protocol, stopping use of the hosted app does not by itself
						delete records from the network.
					</p>
					<p>
						We may update these Terms by posting a revised version on this page. Your continued use
						of the hosted service after an update takes effect means you accept the revised Terms.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">10. Disclaimers</h2>
					<p>
						To the maximum extent permitted by law, Currents is provided on an as-is and
						as-available basis. We do not promise uninterrupted availability, permanent retention,
						perfect accuracy, or that third-party infrastructure will always behave as expected.
					</p>
					<p>
						To the maximum extent permitted by law, we are not liable for indirect, incidental,
						special, consequential, or punitive damages arising from your use of the hosted service.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">11. Contact</h2>
					<p>
						For questions about these Terms or the hosted Currents service, open an issue at
						<a
							class="underline underline-offset-4"
							href="https://github.com/matteomarjanovic/currents/issues"
							target="_blank"
							rel="noreferrer"
						>
							github.com/matteomarjanovic/currents/issues
						</a>
						.
					</p>
				</section>
			</Card.Content>
		</Card.Root>
	</div>
</div>
