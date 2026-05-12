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
	import SaveAttributionDialog from '$lib/components/save-attribution-dialog.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ArrowDown from '@lucide/svelte/icons/arrow-down';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import { getImageContent, type SaveAttribution, type SaveView } from '$lib/types';

	interface Props {
		save: SaveView;
		onClose?: () => void;
	}

	let { save, onClose }: Props = $props();
	let hydratedSave = $state<SaveView | null>(null);
	let currentSave = $derived(hydratedSave ?? save);
	let image = $derived(getImageContent(currentSave));
	let attributionDialogOpen = $state(false);

	let viewerAttr = $derived(currentSave.viewer?.attribution);
	let originalAttr = $derived(image?.attribution);
	let canAttribute = $derived(!!auth.user && (currentSave.viewer?.saves?.length ?? 0) > 0);
	let isOwnSave = $derived(auth.user?.did === currentSave.author.did);
	let hasViewerAttr = $derived(
		!!viewerAttr && (!!viewerAttr.credit || !!viewerAttr.license || !!viewerAttr.url)
	);
	let hasOriginalAttr = $derived(
		!!originalAttr && (!!originalAttr.credit || !!originalAttr.license || !!originalAttr.url)
	);
	let attrsDiffer = $derived(
		hasViewerAttr &&
			hasOriginalAttr &&
			(viewerAttr!.credit !== originalAttr!.credit ||
				viewerAttr!.license !== originalAttr!.license ||
				viewerAttr!.url !== originalAttr!.url)
	);
	let showDual = $derived(!isOwnSave && hasOriginalAttr && hasViewerAttr && attrsDiffer);

	$effect(() => {
		void save.uri;
		hydratedSave = null;
	});

	$effect(() => {
		const uri = save.uri;
		if (!auth.checked || !auth.user || save.viewer?.saves !== undefined) return;

		let cancelled = false;
		void (async () => {
			try {
				const params = new URLSearchParams({ uris: uri });
				const res = await fetch(`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getSaves?${params}`, {
					credentials: 'include'
				});
				if (!res.ok) return;
				const data = (await res.json()) as { saves?: SaveView[] };
				const fetchedSave = data.saves?.[0];
				if (!cancelled && fetchedSave?.uri === uri) {
					hydratedSave = fetchedSave;
				}
			} catch (error) {
				console.error('hydrate save failed', error);
			}
		})();

		return () => {
			cancelled = true;
		};
	});

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

	let authorName = $derived(currentSave.author.displayName || currentSave.author.handle);

	const related = useInfiniteScroll(async (cursor) => {
		const params = new URLSearchParams({ uri: currentSave.uri, limit: '50' });
		if (cursor) params.set('cursor', cursor);
		const res = await fetch(
			`${PUBLIC_APPVIEW_URL}/xrpc/is.currents.feed.getRelatedSaves?${params}`,
			{ credentials: 'include' }
		);
		const data = await res.json();
		return { items: data.saves ?? [], cursor: data.cursor };
	});

	$effect(() => {
		void currentSave.uri;
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

	function handleSavesChange(saves: { collectionUri: string; saveUri: string }[]) {
		const base = hydratedSave ?? save;
		hydratedSave = {
			...base,
			viewer: { ...(base.viewer ?? {}), saves }
		};
	}
</script>

{#snippet attributionFields(attr: SaveAttribution)}
	{#if attr.credit}
		<span>Credit: {attr.credit}</span>
	{/if}
	{#if attr.license}
		<span>License: {attr.license}</span>
	{/if}
	{#if attr.url}
		<a
			href={attr.url}
			target="_blank"
			rel="noopener noreferrer"
			class="inline-flex items-center gap-1 hover:text-foreground"
		>
			<ExternalLink class="size-3" />
			<span class="truncate">Attribution link</span>
		</a>
	{/if}
{/snippet}

{#snippet info()}
	{#if currentSave.text}
		<p class="text-sm whitespace-pre-wrap">{currentSave.text}</p>
	{/if}

	{#if currentSave.originUrl}
		<div class="inline-flex items-center gap-1 text-sm text-muted-foreground">
			<span>Source:</span>
			<a
				href={currentSave.originUrl}
				target="_blank"
				rel="noopener noreferrer"
				class="inline-flex items-center gap-1 hover:text-foreground"
			>
				<span class="truncate">{new URL(currentSave.originUrl).hostname}</span>
				<ExternalLink class="size-3.5" />
			</a>
		</div>
	{/if}

	{#if showDual}
		<div class="flex flex-col gap-1 text-xs text-muted-foreground">
			<span class="font-medium text-foreground/80">Original attribution</span>
			{@render attributionFields(originalAttr!)}
		</div>
		<div class="flex flex-col gap-1 text-xs text-muted-foreground">
			<span class="font-medium text-foreground/80">Your attribution</span>
			{@render attributionFields(viewerAttr!)}
		</div>
	{:else if hasViewerAttr}
		<div class="flex flex-col gap-1 text-xs text-muted-foreground">
			{@render attributionFields(viewerAttr!)}
		</div>
	{:else if hasOriginalAttr}
		<div class="flex flex-col gap-1 text-xs text-muted-foreground">
			{@render attributionFields(originalAttr!)}
		</div>
	{/if}

	{#if canAttribute}
		<Button
			variant="link"
			size="sm"
			class="self-start px-0 text-xs text-muted-foreground hover:text-foreground"
			onclick={() => (attributionDialogOpen = true)}
		>
			{hasViewerAttr ? 'Edit attribution' : '+ Add attribution'}
		</Button>
	{/if}
{/snippet}

{#snippet saveControl(variant: 'popover' | 'drawer')}
	{#if auth.user && collections.loaded}
		<CollectionSelector item={currentSave} {variant} onSavesChange={handleSavesChange} />
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
				alt={currentSave.text ?? ''}
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

<div class="flex flex-col gap-4 p-2 md:hidden">
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
			alt={currentSave.text ?? ''}
			class="w-full"
			style={`${image.width && image.height ? `aspect-ratio: ${image.width} / ${image.height};` : ''}${image.dominantColor ? ` background-color: ${image.dominantColor};` : ''}`}
		/>
	{:else}
		<div
			class="flex items-center justify-center bg-muted text-sm text-muted-foreground"
			style="aspect-ratio: 3 / 4;"
		>
			Unsupported content
		</div>
	{/if}
</div>

{#if related.items.length > 0 || related.loading}
	<section class="flex flex-col gap-4 p-2 md:p-6">
		<h2 class="text-lg font-medium">Related</h2>
		<MasonryGrid items={related.items} loading={related.loading} />
		{#if related.hasMore}
			<div bind:this={sentinel} class="h-1"></div>
		{/if}
	</section>
{/if}

{#if canAttribute}
	<SaveAttributionDialog
		bind:open={attributionDialogOpen}
		save={currentSave}
		onSaved={(attr) => {
			const base = hydratedSave ?? save;
			hydratedSave = {
				...base,
				viewer: { ...(base.viewer ?? {}), attribution: attr }
			};
		}}
	/>
{/if}
