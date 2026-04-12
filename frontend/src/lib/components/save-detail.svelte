<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import * as Avatar from '$lib/components/ui/avatar';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import type { SaveView } from '$lib/types';

	interface Props {
		save: SaveView;
		onClose?: () => void;
	}

	let { save, onClose }: Props = $props();

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
</script>

{#snippet info()}
	<div class="flex items-center gap-3">
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
	</div>

	{#if save.text}
		<p class="text-sm whitespace-pre-wrap">{save.text}</p>
	{/if}

	{#if save.originUrl}
		<a
			href={save.originUrl}
			target="_blank"
			rel="noopener noreferrer"
			class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
		>
			<ExternalLink class="size-3.5" />
			<span class="truncate">Source</span>
		</a>
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
	<div class="flex w-1/3 flex-col gap-5 overflow-y-auto p-6">
		<Button variant="ghost" size="sm" class="w-fit" onclick={goBack}>
			<ArrowLeft class="size-4" />
			Back
		</Button>
		{@render info()}
		<div class="mt-auto">
			{@render saveControl('popover')}
		</div>
	</div>
	<div class="flex w-2/3 items-center justify-center p-6">
		<img src={save.imageUrl} alt={save.text ?? ''} class="max-h-full max-w-full object-contain" />
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
	<img
		src={save.imageUrl}
		alt={save.text ?? ''}
		class="w-full rounded-lg"
		style={save.width && save.height ? `aspect-ratio: ${save.width} / ${save.height}` : undefined}
	/>
</div>
