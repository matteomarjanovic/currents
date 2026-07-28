<script lang="ts">
	import { page } from '$app/state';
	import { STANDARD_SITE_DID } from '$lib/standard-site';

	let {
		title,
		description,
		date,
		standardSiteRkey,
		children
	}: {
		title: string;
		description: string;
		date: string;
		standardSiteRkey?: string;
		children: import('svelte').Snippet;
	} = $props();

	const dateFormat = new Intl.DateTimeFormat('en-US', { dateStyle: 'long' });
	// Generated at build time by scripts/gen-og-images.mjs from this same title, into
	// static/og/<slug>.png — slug is just the last path segment, matching the post's folder name.
	let ogImage = $derived(`https://currents.is/og/${page.url.pathname.split('/').at(-1)}.png`);
</script>

<svelte:head>
	<title>{title} · Currents Blog</title>
	<meta name="description" content={description} />
	<meta property="og:type" content="article" />
	<meta property="og:title" content={title} />
	<meta property="og:description" content={description} />
	<meta property="og:image" content={ogImage} />
	<meta property="og:image:width" content="1200" />
	<meta property="og:image:height" content="630" />
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:image" content={ogImage} />
	{#if standardSiteRkey}
		<!-- Standard.site verification for this document — https://standard.site/docs/verification.
		     Juttu (comments) reads this same tag via an exact `rel="site.standard.document"` selector
		     to resolve which Bluesky thread to show, so the rel value can't have anything added to
		     it — svelte.config.js's handleInvalidUrl keeps the prerender crawler from choking on the
		     at:// href (a valid AT-URI, but not a parseable WHATWG URL) instead.
		     https://juttu.app/getting-started/installation/ -->
		<link
			rel="site.standard.document"
			href="at://{STANDARD_SITE_DID}/site.standard.document/{standardSiteRkey}"
		/>
		<script
			defer
			src="https://cdn.jsdelivr.net/npm/juttu@latest/juttu-embed.js"
			// src="https://designed-favourite-endless-longest.trycloudflare.com/embed/juttu-embed.js"
			// data-api-url="https://designed-favourite-endless-longest.trycloudflare.com"
		></script>
	{/if}
</svelte:head>

<article class="flex flex-col gap-6">
	<header class="flex flex-col gap-2">
		<h1 class="text-3xl leading-tight font-semibold text-foreground">{title}</h1>
		<p class="text-sm text-muted-foreground">{dateFormat.format(new Date(date))}</p>
	</header>

	<div class="prose max-w-none prose-neutral dark:prose-invert">
		{@render children()}
	</div>

	{#if standardSiteRkey}
		<div id="juttu-comments"></div>
	{/if}
</article>

<style>
	/* Maps Juttu's theming variables (https://juttu.app/getting-started/customization/) onto our
	   own design tokens, so the widget follows the site's light/dark theme and typography instead
	   of its own defaults. `:global` is required — Juttu injects `.juttu-comments` itself, so it
	   never carries Svelte's scoping class.
	   `!important` is required too: Juttu's own stylesheet sets these same variables again on
	   `.juttu-comments[data-juttu-theme="dark"/"light"]`, which outranks our plain class selector
	   regardless of stylesheet order, so a normal declaration here is silently dropped. */
	:global(.juttu-comments) {
		--juttu-font-family: var(--font-sans) !important;
		--juttu-surface: var(--input) !important;
		--juttu-border-color: var(--border) !important;
		--juttu-text: var(--foreground) !important;
		--juttu-text-muted: var(--muted-foreground) !important;
		--juttu-accent-color: var(--primary) !important;
		--juttu-link-color: var(--primary) !important;
		--juttu-radius: var(--radius) !important;
		--juttu-autocomplete-bg: var(--popover) !important;
	}

	/* Juttu's filled CTA buttons hardcode `color: #fff` instead of reading a variable, so pairing
	   `--juttu-accent-color` above with our (theme-inverting) `--primary` would make them unreadable
	   in dark mode — restyle them directly instead, matching our own Button "default" variant. */
	.prose :global(img),
	.prose :global(video) {
		width: 100%;
		border-radius: var(--radius-2xl);
	}

	:global(.juttu-linking-start-btn),
	:global(.juttu-linking-continue-btn),
	:global(.juttu-linking-login-btn),
	:global(.juttu-submit-btn),
	:global(.juttu-reply-submit) {
		background: var(--primary) !important;
		color: var(--primary-foreground) !important;
		border-radius: var(--radius-4xl) !important;
	}

	/* The active sort badge (newest/oldest/top) has the same hardcoded `color: #fff` problem: it
	   pairs white text with a `--juttu-accent-color` (= our theme-inverting `--primary`) background,
	   so it's white-on-light and unreadable in dark mode. Use our own foreground token instead. */
	:global(.juttu-sort-btn--active) {
		color: var(--primary-foreground) !important;
	}
</style>
