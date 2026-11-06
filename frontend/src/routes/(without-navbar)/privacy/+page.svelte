<script lang="ts">
	import { resolve } from '$app/paths';
	import LogoMerged from '$lib/assets/logo.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import SiteFooter from '$lib/components/site-footer.svelte';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
</script>

<svelte:head>
	<title>Privacy Policy · Currents</title>
	<meta
		name="description"
		content="Privacy Policy for the hosted Currents web app, mobile apps, browser extension, APIs, and related services."
	/>
</svelte:head>

<div class="min-h-svh px-4 py-8 md:px-8">
	<div class="mx-auto flex max-w-3xl flex-col gap-6">
		<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<a href={resolve('')} class="flex h-5 items-center gap-2 font-medium">
				<LogoMerged />
			</a>
			<div class="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
				<a class="underline underline-offset-4" href={resolve('terms')}>Terms</a>
				<a class="underline underline-offset-4" href={resolve('privacy')}>Privacy</a>
				<a class="underline underline-offset-4" href={resolve('refunds')}>Refunds</a>
				<a class="underline underline-offset-4" href={resolve('')}>Go to app</a>
				<ThemeToggle />
			</div>
		</div>

		<Card.Root>
			<Card.Header class="gap-3">
				<Card.Title class="text-3xl leading-tight">Privacy Policy</Card.Title>
				<Card.Description>
					Last updated November 6, 2026. This policy explains how the hosted Currents service
					collects, uses, and shares information.
				</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-8 text-sm leading-6 text-foreground/90 md:text-[15px]">
				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">1. Scope</h2>
					<p>
						The hosted Currents service is operated by Matteo Marjanovic (VAT: IT05179170237), who
						is responsible for the personal data processing described in this policy. You can
						contact us at
						<a class="underline underline-offset-4" href="mailto:privacy@currents.is"
							>privacy@currents.is</a
						>.
					</p>
					<p>
						This policy applies to the hosted Currents web app, iOS and Android apps, browser
						extension, APIs, appview, and related search and inference infrastructure that we
						operate. It covers how we obtain and use public AT Protocol records, including records
						published through other clients. Self-hosted forks and third parties such as your
						Personal Data Server (PDS), account provider, and linked websites have their own
						policies for the processing they control.
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
							Public content you create or interact with, including images, collections, saves, alt
							text, captions, notes, source URLs, attribution, and references to source AT Protocol
							records. We also index public follows and favourited collections linked to your
							account.
						</li>
						<li>
							Derived metadata used to operate discovery features, including image embeddings,
							dominant-color palettes, image dimensions, save clustering signals, and collection
							embeddings. Automated image analysis also produces content scores and labels for
							moderation, AI-generated content, and discovery-feed filtering.
						</li>
						<li>
							Authentication data needed to keep you signed in, including a signed
							<code>currents-session</code> cookie in the browser, an equivalent session token kept in
							your device's secure storage in the mobile apps, and server-side OAuth session records.
							On iOS, the app also shares authentication data locally with its share extension so you
							can save without signing in again.
						</li>
						<li>
							Search and interaction data you submit to the service, such as text queries for
							semantic search, query colors, and collection filters. Free color-search trials store
							the colors used, your account identifier, and when each trial was used.
						</li>
						<li>
							Account preferences and interaction state stored on our servers, including pinned
							collections, feed and moderation settings, hidden feed images, saving preferences, and
							which feature announcements and notifications you have dismissed or seen.
						</li>
						<li>
							Pinterest import information, including the public boards you select, destination
							collections, source image URLs, progress, errors, and import history. Imports use
							anonymous Pinterest sessions and do not require your Pinterest password or login
							cookies.
						</li>
						<li>
							Moderation reports, including the reporter's account identifier, reported content,
							reason, and review history. If you contact us for support or a privacy request, we
							also receive your contact details and the information you provide.
						</li>
						<li>
							Request and diagnostic information needed to deliver, secure, and debug the service,
							such as IP addresses, browser or app information, timestamps, and error details.
							Infrastructure providers may retain this information in operational or security logs.
						</li>
						<li>
							Local app and browser state, such as your last-used collection, theme, temporary
							sign-in flow state, and cached content. The extension also stores its save-button
							settings locally.
						</li>
						<li>
							If you use the browser extension, it may access the current page URL, page title,
							image URLs or bytes, and Currents session cookie to prepare and create saves. It
							inspects page images locally to display hover save buttons on enabled sites, including
							Pinterest by default, and to populate the image picker when you open it. Opening the
							clipper or selecting an image can also fetch image data and supported source-record
							metadata. Publishing a save requires your action; local page inspection does not
							upload your general browsing history to Currents.
						</li>
						<li>
							In the mobile apps, the images you choose to save: pictures you pick from your photo
							library or capture with your camera. Camera and photo access are used only when you
							actively select or take an image to save, never in the background.
						</li>
						<li>
							Content shared to the mobile app from other apps (an image or link you send to
							Currents through the system share sheet), used only to pre-fill a save you choose to
							create.
						</li>
						<li>
							Subscription and billing data if you become a Supporter. Checkout and billing are
							handled by Polar, our merchant of record. Depending on the checkout and tax
							requirements, Polar collects billing contact details, such as your name, email,
							address and country, and payment information. It may use cookies and process request
							metadata in the checkout window. We send Polar your DID to associate the subscription
							with your Currents account. Our subscription database stores your status, plan,
							relevant dates, and Polar customer and subscription identifiers, not your full payment
							card details. Billing information may also be available to us through Polar's
							dashboard or API.
						</li>
						<li>
							Usage analytics collected with Umami on the web and in the mobile apps, including page
							views and events such as sign-in attempts, feature paywalls, and checkout starts or
							completions. Events can include the platform, account-provider category, and
							subscription plan. We do not supply your name, email, DID, or handle as an analytics
							identifier. Page titles are omitted, search terms and content identifiers are removed
							from analytics URLs, and external referrers are reduced to their website origin.
						</li>
					</ul>
					<p>
						Umami uses no analytics cookies, but can group activity using a derived visitor or
						session identifier based on an IP address, browser information, and a rotating salt.
						Analytics can include browser, operating system, screen size, language, referral and
						campaign information, and approximate location derived from the IP address. These
						records are not necessarily anonymous just because they lack your account identifier.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">3. How we use information</h2>
					<p>We use information to:</p>
					<ul class="list-disc space-y-2 pl-5 marker:text-muted-foreground">
						<li>Authenticate you and maintain your session.</li>
						<li>Fetch, index, display, and synchronize public AT Protocol content.</li>
						<li>Provide search, related saves, discovery feeds, and personalization features.</li>
						<li>Generate technical derivatives like embeddings and image metadata.</li>
						<li>Synchronize your preferences, follow graph, and library across devices.</li>
						<li>
							Run imports, track trial allowances, and provide Supporter access and billing support.
						</li>
						<li>
							Measure feature use, audience size, and sign-in and checkout flows to improve
							Currents.
						</li>
						<li>Detect abuse, secure the service, debug failures, and maintain reliability.</li>
						<li>Comply with legal obligations and enforce our Terms.</li>
					</ul>
					<p>
						Where EU or UK data protection law applies, we rely on performance of our contract with
						you for account access and the features you request; our legitimate interests in
						operating, securing, moderating, and improving the service, including measuring its use;
						and compliance with legal obligations where applicable. Where processing requires your
						consent, consent is the relevant legal basis and can be withdrawn. Account identifiers
						and authentication are necessary for account features; optional profile fields and
						content are provided at your choice.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">4. What is public</h2>
					<p>
						Currents is designed around public AT Protocol records. If you publish content through
						Currents, your profile, images, collections, saves, follows, favourited collections, and
						related metadata may be visible in Currents, on your PDS, and through other clients or
						protocol consumers. Unsorted saves on your profile are public too. Content labels we
						issue can be distributed through public AT Protocol label endpoints.
					</p>
					<p>
						Search queries, trial ledgers, account settings, pinned collections, hidden feed images,
						import progress, authentication data, analytics, and the contents of moderation reports
						are not published as public user records. Moderators can access reports and review data.
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
							With infrastructure providers that host and deliver the service. Our main application,
							database, and inference services run on Scaleway in the Paris region, and Bunny
							delivers images through our CDN. Image delivery providers receive the requests needed
							to serve you content, including network metadata such as your IP address.
						</li>
						<li>
							With the infrastructure hosting our Umami analytics instance, currently served through
							Netlify, to process the usage analytics described above. This is separate from your
							Currents account database. See
							<a
								class="underline underline-offset-4"
								href="https://docs.umami.is/docs/metric-definitions"
								target="_blank"
								rel="noreferrer">Umami's data documentation</a
							>.
						</li>
						<li>
							With Polar, our payments provider and merchant of record, when you purchase or manage
							a Supporter subscription, including your DID and selected plan. Polar processes your
							billing details and checkout activity under its own
							<a
								class="underline underline-offset-4"
								href="https://polar.sh/legal/privacy-policy"
								target="_blank"
								rel="noreferrer">privacy policy</a
							>.
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
						We do not sell personal information or use your Currents profile, saves, or interaction
						data to serve targeted ads. Analytics is used to understand our own service. Third-party
						checkout and linked services have their own data practices, as described in their
						policies.
					</p>
					<p>
						Providers processing personal data on our behalf must protect it in accordance with this
						policy and applicable data protection requirements. PDS operators, Polar, and other
						services may also act independently for processing they control. Public AT Protocol
						content is accessible worldwide, and third-party services and CDN infrastructure may
						process information outside the European Economic Area. Contact us for information about
						the recipients, processing locations, and applicable transfer safeguards.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">6. Retention</h2>
					<p>
						We retain indexed public content and its search metadata to display saves and provide
						search and discovery. Removing content from your PDS or deleting an account removes the
						corresponding live account data as described below; changes may take time to reach
						indexes and caches. Derived metadata for an image may remain when other users still save
						that image or it is still used by the discovery system.
					</p>
					<p>
						OAuth sessions are configured to expire after 90 days or 14 days of inactivity, with
						cleanup performed when sessions are accessed. Account deletion removes sessions except
						for one temporarily kept to complete an optional PDS deletion job.
					</p>
					<p>
						Account preferences, trial colors, and import history are retained until account
						deletion or removal through an available feature. Completed import history is kept to
						avoid importing the same images again, including images you later move or delete. Trial
						colors remain on record after the free allowance is used up, until account deletion.
					</p>
					<p>
						Currents keeps its subscription records while your account exists and removes them on
						account deletion. Polar keeps its own billing and transaction records under its
						retention policy, including any accounting, tax, fraud-prevention, or dispute
						requirements.
					</p>
					<p>
						Moderation reports, labels, and review or audit history are not automatically removed by
						account deletion. They are retained to maintain moderation history, prevent abuse, and
						handle disputes or legal obligations. Analytics and infrastructure logs are stored
						separately and follow their configured retention settings; deleting an account does not
						automatically erase those records. Contact us about retention or deletion of these data.
					</p>
					<p>
						Routine database backups in our Scaleway backup bucket are configured to expire after 30
						days. Account deletion does not edit existing backup copies. Browser and app storage can
						remain until you clear it or the app replaces it; logging out clears the active local
						authentication data.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">7. Your choices</h2>
					<p>
						You can request account deletion from <strong
							>Settings → Account → Delete account</strong
						>. This removes your profile, collections, saves, outgoing follows, favourites,
						preferences, trial ledger, import history, and Currents' subscription records, and stops
						indexing your account. Other users' records, such as their follows of your account,
						favourites, or copies of images, remain theirs. Moderation and audit history is retained
						as explained above.
					</p>
					<p>
						By default, your original Currents records remain on your PDS, and signing in again can
						restore them to Currents. Select the additional option to delete Currents records from
						your AT Protocol repo if you want those removed too. That deletion runs in the
						background, can take longer for large libraries or when your PDS limits requests, and
						requires a working authorization. Signing in again while it runs stops the remaining
						deletion. It does not delete your AT Protocol account or other apps' records, and cannot
						erase copies already held by other users or independent services.
					</p>
					<p>
						The current self-service deletion flow is unavailable while a Supporter subscription
						still grants access, including a canceled subscription through the end of its paid
						period. Manage billing under <strong>Settings → Subscription</strong>. If you need help
						with deletion, including while a subscription is active, email
						<a class="underline underline-offset-4" href="mailto:privacy@currents.is"
							>privacy@currents.is</a
						>.
					</p>
					<ul class="list-disc space-y-2 pl-5 marker:text-muted-foreground">
						<li>
							Review or change your public profile and records through your AT Protocol account.
						</li>
						<li>
							Log out to clear the active Currents session: the cookie on the web, or the stored
							session token on mobile.
						</li>
						<li>Change feed, saving, and moderation preferences in Settings.</li>
						<li>Clear local browser storage or app data through your device's settings.</li>
						<li>
							Web analytics is configured to honor your browser's Do Not Track preference. There is
							currently no separate in-app analytics switch. Contact us with analytics privacy
							requests or to object to processing.
						</li>
						<li>Avoid posting private, sensitive, or confidential material to Currents.</li>
						<li>
							Control the sites where extension hover buttons appear in the extension's settings, or
							disable the extension through your browser.
						</li>
					</ul>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">8. Your privacy rights</h2>
					<p>
						Depending on the law that applies to you, you may request access to, correction of,
						deletion of, or a portable copy of your personal data, restrict processing, or object to
						processing based on legitimate interests. Where processing is based on consent, you can
						withdraw that consent without affecting processing that occurred before withdrawal. Send
						requests to
						<a class="underline underline-offset-4" href="mailto:privacy@currents.is"
							>privacy@currents.is</a
						>. We may need to verify that you control the relevant account. You can also lodge a
						complaint with your local data protection authority; in Italy, this is the
						<a
							class="underline underline-offset-4"
							href="https://www.garanteprivacy.it/"
							target="_blank"
							rel="noreferrer">Garante per la protezione dei dati personali</a
						>.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">9. Children's privacy</h2>
					<p>
						Currents is not intended for children under 13 or the higher minimum age required by
						local law. If you believe a child has provided personal information in violation of this
						policy, contact us using the method below.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">10. Changes to this policy</h2>
					<p>
						We may update this policy from time to time. When we do, we will post the revised
						version here and update the date at the top of the page. Where a change requires notice
						or consent, we will provide that notice or request consent as required by law. Continued
						use alone does not replace consent where consent is required.
					</p>
				</section>

				<section class="space-y-3">
					<h2 class="text-lg font-semibold text-foreground">11. Contact</h2>
					<p>
						For privacy questions or requests related to the hosted Currents service, including data
						deletion requests, email
						<a class="underline underline-offset-4" href="mailto:privacy@currents.is"
							>privacy@currents.is</a
						>. For general product issues, you can also use
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
					<p>GitHub issues are public. Please use email for personal data or account details.</p>
				</section>
			</Card.Content>
		</Card.Root>
	</div>
</div>

<SiteFooter />
