<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { Button } from '$lib/components/ui/button';
	import * as Avatar from '$lib/components/ui/avatar';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ArrowDown from '@lucide/svelte/icons/arrow-down';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import { getImageContent, type SaveView } from '$lib/types';

	interface Props {
		save: SaveView;
		onClose?: () => void;
	}

	let { save, onClose }: Props = $props();
	let image = $derived(getImageContent(save));

	function goBack() {
		if (onClose) {
			onClose();
			return;
		}
		if (typeof history !== 'undefined' && history.length > 1) {
			history.back();
		} else {
			goto('/');
		}
	}

	let authorName = $derived(save.author.displayName || save.author.handle);

	const related = useInfiniteScroll(async (cursor) => {
		const params = new URLSearchParams({ uri: save.uri, limit: '50' });
		if (cursor) params.set('cursor', cursor);
		const res = await fetch(
			`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getRelatedSaves?${params}`,
			{ credentials: 'include' }
		);
		const data = await res.json();
		return { items: data.saves ?? [], cursor: data.cursor };
	});

	$effect(() => {
		void save.uri;
		untrack(() => {
			related.reset();
			related.loadMore();
		});
	});

	let sentinel: HTMLDivElement | undefined = $state();

	$effect(() => {
		if (!sentinel) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) related.loadMore();
			},
			{ rootMargin: '400px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});
</script>

{#snippet info()}
	<!-- <div class="flex items-center gap-3">
		<Avatar.Root class="size-10">
			{#if save.author.avatar}
				<Avatar.Image src={save.author.avatar} alt={authorName} />
			{/if}
			<Avatar.Fallback>{(authorName || '?').slice(0, 1).toUpperCase()}</Avatar.Fallback>
		</Avatar.Root>
		<div class="flex min-w-0 flex-col">
			<span class="truncate text-sm font-medium">{authorName}</span>
			{#if save.author.handle}
				<span class="truncate text-xs text-muted-foreground">@{save.author.handle}</span>
			{/if}
		</div>
	</div> -->

	{#if save.text}
		<p class="text-sm whitespace-pre-wrap">{save.text}</p>
	{/if}

	{#if save.originUrl}
		<div class="inline-flex items-center gap-1 text-sm text-muted-foreground">
			<span>Source:</span>
			<a
				href={save.originUrl}
				target="_blank"
				rel="noopener noreferrer"
				class="inline-flex items-center gap-1 hover:text-foreground"
			>
				<span class="truncate">{new URL(save.originUrl).hostname}</span>
				<ExternalLink class="size-3.5" />
			</a>
		</div>
	{/if}

	{#if save.attribution && (save.attribution.credit || save.attribution.license || save.attribution.url)}
		<div class="flex flex-col gap-1 text-xs text-muted-foreground">
			{#if save.attribution.credit}
				<span>Credit: {save.attribution.credit}</span>
			{/if}
			{#if save.attribution.license}
				<span>License: {save.attribution.license}</span>
			{/if}
			{#if save.attribution.url}
				<a
					href={save.attribution.url}
					target="_blank"
					rel="noopener noreferrer"
					class="inline-flex items-center gap-1 hover:text-foreground"
				>
					<ExternalLink class="size-3" />
					<span class="truncate">Attribution link</span>
				</a>
			{/if}
		</div>
	{/if}
{/snippet}

{#snippet saveControl(variant: 'popover' | 'drawer')}
	{#if auth.user && collections.loaded}
		<CollectionSelector item={save} {variant} />
	{:else if auth.checked}
		<Button variant="default" onclick={promptLogin} class="w-full">Save</Button>
	{/if}
{/snippet}

<div class="hidden h-screen md:flex">
	<div class="flex w-1/3 flex-col gap-5 overflow-y-auto border-r border-border p-6">
		<div class="flex items-center justify-between gap-2">
			<Button variant="ghost" size="sm" onclick={goBack}>
				<ArrowLeft class="size-4" />
				Back
			</Button>
			<div class="w-auto min-w-32">
				{@render saveControl('popover')}
			</div>
		</div>
		{@render info()}
		<div class="text-md mt-auto flex flex-col items-center gap-2 text-center text-muted-foreground">
			<p>Scroll down to view related images</p>
			<ArrowDown class="size-4" />
		</div>
	</div>
	<div class="flex w-2/3 items-center justify-center p-6">
		{#if image}
			<img
				src={image.imageUrl}
				alt={save.text ?? ''}
				class="max-h-full max-w-full object-contain"
				style={image.dominantColor ? `background-color: ${image.dominantColor}` : undefined}
			/>
		{:else}
			<div
				class="flex h-full w-full items-center justify-center rounded-lg bg-muted text-sm text-muted-foreground"
			>
				Unsupported content
			</div>
		{/if}
	</div>
</div>

<div class="flex flex-col gap-4 p-4 md:hidden">
	<div class="flex items-center justify-between gap-2">
		<Button variant="ghost" size="sm" onclick={goBack}>
			<ArrowLeft class="size-4" />
			Back
		</Button>
		<div class="w-auto min-w-32">
			{@render saveControl('drawer')}
		</div>
	</div>
	{@render info()}
	{#if image}
		<img
			src={image.imageUrl}
			alt={save.text ?? ''}
			class="w-full rounded-lg"
			style={`${image.width && image.height ? `aspect-ratio: ${image.width} / ${image.height};` : ''}${image.dominantColor ? ` background-color: ${image.dominantColor};` : ''}`}
		/>
	{:else}
		<div
			class="flex items-center justify-center rounded-lg bg-muted text-sm text-muted-foreground"
			style="aspect-ratio: 3 / 4;"
		>
			Unsupported content
		</div>
	{/if}
</div>

{#if related.items.length > 0 || related.loading}
	<section class="flex flex-col gap-4 p-4 md:p-6">
		<h2 class="text-lg font-medium">Related</h2>
		<MasonryGrid items={related.items} loading={related.loading} />
		{#if related.hasMore}
			<div bind:this={sentinel} class="h-1"></div>
		{/if}
	</section>
{/if}
