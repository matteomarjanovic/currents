<script lang="ts">
	import SaveDetail from '$lib/components/save-detail.svelte';
	import { page } from '$app/state';
	import { getImageContent } from '$lib/types';
	import { bunnyImageUrl } from '$lib/image-url';
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
	const ogImage = $derived(
		image
			? bunnyImageUrl(image.imageUrl, {
					width: Math.min(image.width ?? 1200, 1200),
					quality: 85
				})
			: ''
	);
	const ogWidth = $derived(
		image?.width && ogImage !== image.imageUrl ? Math.min(image.width, 1200) : image?.width
	);
	const ogHeight = $derived(
		image?.width && image.height && ogWidth
			? Math.round((image.height * ogWidth) / image.width)
			: image?.height
	);
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
		<meta property="og:image" content={ogImage} />
		{#if ogWidth}<meta property="og:image:width" content={String(ogWidth)} />{/if}
		{#if ogHeight}<meta property="og:image:height" content={String(ogHeight)} />{/if}
		<meta name="twitter:image" content={ogImage} />
	{/if}
	<meta name="twitter:card" content={image ? 'summary_large_image' : 'summary'} />
	<meta name="twitter:title" content={title + ' · Currents'} />
	<meta name="twitter:description" content={description} />
</svelte:head>

<SaveDetail save={data.save} />
