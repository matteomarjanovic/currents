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
			// src="https://cdn.jsdelivr.net/npm/juttu@latest/juttu-embed.js"
			src="https://designed-favourite-endless-longest.trycloudflare.com/embed/juttu-embed.js"
			data-api-url="https://designed-favourite-endless-longest.trycloudflare.com"
		></script>
	{/if}
</svelte:head>

<article class="flex flex-col gap-6">
	<header class="flex flex-col gap-2">
		<a href="/blog" class="text-sm text-muted-foreground underline underline-offset-4">← Blog</a>
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
