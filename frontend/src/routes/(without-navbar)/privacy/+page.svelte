<script lang="ts">
	import { resolve } from '$app/paths';
	import LogoMerged from '$lib/assets/logo.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
</script>

<svelte:head>
	<title>Privacy Policy • Currents</title>
	<meta
		name="description"
		content="Privacy Policy for the hosted Currents web app, browser extension, APIs, and related services."
	/>
</svelte:head>

<div class="min-h-svh bg-muted/40 px-4 py-8 md:px-8">
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
				<Card.Title class="text-3xl leading-tight">Privacy Policy</Card.Title>
				<Card.Description>
					Last updated April 22, 2026. This policy explains how the hosted Currents service
					collects, uses, and shares information.
				</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-8 text-sm leading-6 text-foreground/90 md:text-[15px]">
				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">1. Scope</h2>
					<p>
						This policy applies to the hosted Currents web app, browser extension, APIs, appview,
						and related search and inference infrastructure that we operate. It does not cover
						self-hosted forks or third-party services such as your PDS, Bluesky, Eurosky, or sites
						you visit through links saved in Currents.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">2. Information we collect</h2>
					<p>We collect or process the following categories of information:</p>
					<ul class="list-disc space-y-2 pl-5 marker:text-muted-foreground">
						<li>
							Account and profile data such as your DID, handle, display name, avatar, description,
							pronouns, website, and related public AT Protocol profile fields.
						</li>
						<li>
							Public content you create or interact with through Currents, including collections,
							saves, image blob references, captions, notes, source URLs, and optional attribution
							metadata.
						</li>
						<li>
							Derived metadata used to operate discovery features, including image embeddings,
							dominant-color palettes, image dimensions, save clustering signals, and collection
							embeddings.
						</li>
						<li>
							Authentication data needed to keep you signed in, including a signed
							<code>currents-session</code> cookie in the browser and server-side OAuth session records.
						</li>
						<li>
							Search and interaction data you submit to the service, such as text queries for
							semantic search and standard request metadata needed to deliver, secure, and debug the
							service.
						</li>
						<li>
							Local browser state. The web app currently stores your last-used collection URI in
							local storage to make saving faster.
						</li>
						<li>
							If you use the browser extension, it may access the current page URL, page title,
							selected image URL or bytes, and Currents session cookie so it can create a save only
							when you invoke the extension.
						</li>
					</ul>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">3. How we use information</h2>
					<p>We use information to:</p>
					<ul class="list-disc space-y-2 pl-5 marker:text-muted-foreground">
						<li>Authenticate you and maintain your session.</li>
						<li>Fetch, index, display, and synchronize public AT Protocol content.</li>
						<li>Provide search, related saves, discovery feeds, and personalization features.</li>
						<li>Generate technical derivatives like embeddings and image metadata.</li>
						<li>Detect abuse, secure the service, debug failures, and maintain reliability.</li>
						<li>Comply with legal obligations and enforce our Terms.</li>
					</ul>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">4. What is public</h2>
					<p>
						Currents is designed around public AT Protocol records. If you publish content through
						Currents, that content and related metadata may be visible in Currents, on your PDS, and
						through other clients or protocol consumers.
					</p>
					<p>
						Your search queries, session data, and the local browser state described above are not
						intended to be public user content.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">5. How we share information</h2>
					<p>We may share information in the following ways:</p>
					<ul class="list-disc space-y-2 pl-5 marker:text-muted-foreground">
						<li>
							With the AT Protocol ecosystem and your chosen identity or hosting providers when
							necessary to authenticate you, fetch records, upload data, or serve public content.
						</li>
						<li>
							With infrastructure and service providers that help us host the database, appview,
							image proxy, and inference systems on our behalf.
						</li>
						<li>
							With other users and the public when the information is part of public AT Protocol
							content.
						</li>
						<li>
							If required by law, to protect users or the service, or as part of a reorganization,
							asset transfer, or similar transaction.
						</li>
					</ul>
					<p>
						We do not currently run third-party advertising trackers on the web app, sell personal
						information, or use Currents data to serve ads.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">6. Retention</h2>
					<p>
						Public records and their derived search metadata may remain in our indexes for as long
						as needed to operate the hosted service, plus reasonable backup and recovery windows.
						Deleting content from your PDS or through Currents may take time to propagate through
						indexes and caches.
					</p>
					<p>
						Hosted OAuth sessions are kept only as long as needed to keep you signed in. At the time
						of writing, Currents expires sessions after roughly 90 days and may clear them earlier
						after extended inactivity.
					</p>
					<p>
						Browser local storage remains on your device until you clear it or the app overwrites
						it.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">7. Your choices</h2>
					<ul class="list-disc space-y-2 pl-5 marker:text-muted-foreground">
						<li>
							Review or change your public profile and records through your AT Protocol account.
						</li>
						<li>Log out to clear the active Currents session cookie from the hosted web app.</li>
						<li>Clear local browser storage in your browser settings.</li>
						<li>Avoid posting private, sensitive, or confidential material to Currents.</li>
						<li>
							Only use the browser extension when you want to clip a page or image; it is not meant
							to monitor browsing in the background.
						</li>
					</ul>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">8. Children's privacy</h2>
					<p>
						Currents is not intended for children under 13 or the higher minimum age required by
						local law. If you believe a child has provided personal information in violation of this
						policy, contact us using the method below.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">9. Changes to this policy</h2>
					<p>
						We may update this policy from time to time. When we do, we will post the revised
						version here and update the date at the top of the page. Your continued use of the
						hosted service after the update takes effect means the revised policy applies.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">10. Contact</h2>
					<p>
						For privacy questions or requests related to the hosted Currents service, open an issue
						at
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
