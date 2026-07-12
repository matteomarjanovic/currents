<script lang="ts">
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
</script>

<svelte:head>
	<title>{title} · Currents Blog</title>
	<meta name="description" content={description} />
	<meta property="og:type" content="article" />
	<meta property="og:title" content={title} />
	<meta property="og:description" content={description} />
	{#if standardSiteRkey}
		<!-- Standard.site verification for this document — https://standard.site/docs/verification -->
		<link
			rel="site.standard.document"
			href="at://{STANDARD_SITE_DID}/site.standard.document/{standardSiteRkey}"
		/>
	{/if}
</svelte:head>

<article class="flex flex-col gap-6">
	<header class="flex flex-col gap-2">
		<a href="/blog" class="text-sm text-muted-foreground underline underline-offset-4">← Blog</a>
		<h1 class="text-3xl font-semibold leading-tight text-foreground">{title}</h1>
		<p class="text-sm text-muted-foreground">{dateFormat.format(new Date(date))}</p>
	</header>

	<div class="prose prose-neutral dark:prose-invert max-w-none">
		{@render children()}
	</div>
</article>
