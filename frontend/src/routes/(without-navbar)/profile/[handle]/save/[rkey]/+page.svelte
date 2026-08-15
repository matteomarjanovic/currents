<script lang="ts">
	import SaveDetail from '$lib/components/save-detail.svelte';
	import { page } from '$app/state';
	import { getImageContent } from '$lib/types';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const author = $derived(data.save.author.displayName || '@' + data.save.author.handle);
	const title = $derived((data.save.text?.trim() || 'Save by ' + author).slice(0, 70));
	const image = $derived(getImageContent(data.save));
	const description = $derived(
		(
			data.save.text?.trim() ||
			image?.alt?.trim() ||
			`An image saved by ${author} on Currents.`
		).slice(0, 200)
	);
	const canonical = $derived(page.url.origin + page.url.pathname);
</script>

<svelte:head>
	<title>{title + ' · Currents'}</title>
	<link rel="canonical" href={canonical} />
	<meta name="description" content={description} />
	<meta property="og:type" content="article" />
	<meta property="og:url" content={canonical} />
	<meta property="og:title" content={title + ' · Currents'} />
	<meta property="og:description" content={description} />
	{#if image}
		<meta property="og:image" content={image.imageUrl} />
		{#if image.width}<meta property="og:image:width" content={String(image.width)} />{/if}
		{#if image.height}<meta property="og:image:height" content={String(image.height)} />{/if}
		<meta name="twitter:image" content={image.imageUrl} />
	{/if}
	<meta name="twitter:card" content={image ? 'summary_large_image' : 'summary'} />
	<meta name="twitter:title" content={title + ' · Currents'} />
	<meta name="twitter:description" content={description} />
</svelte:head>

<SaveDetail save={data.save} />
