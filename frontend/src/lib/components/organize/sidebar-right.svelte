<script lang="ts">
	import { linear } from 'svelte/easing';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Badge, badgeVariants } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { cn } from '$lib/utils.js';
	import LabeledMedia from '$lib/components/labeled-media.svelte';
	import SimilarPanel from '$lib/components/organize/similar-panel.svelte';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import { favouriteCollections } from '$lib/stores/favourites.svelte';
	import { getImageContent, type SaveView } from '$lib/types';
	import { copyLink, copyImage, downloadImage } from '$lib/save-actions';
	import X from '@lucide/svelte/icons/x';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import Plus from '@lucide/svelte/icons/plus';
	import Sparkles from '@lucide/svelte/icons/sparkles';
	import Copy from '@lucide/svelte/icons/copy';
	import LinkIcon from '@lucide/svelte/icons/link';
	import Download from '@lucide/svelte/icons/download';

	let {
		save,
		onClose,
		onSavesChange,
		onFindSimilar
	}: {
		save: SaveView;
		onClose: () => void;
		onSavesChange?: (saves: { collectionUri: string; saveUri: string }[]) => void;
		onFindSimilar: (save: SaveView) => void;
	} = $props();

	const sidebar = Sidebar.useSidebar();

	let tab = $state('details');

	let image = $derived(getImageContent(save));
	let palette = $derived(image?.palette ?? (image?.dominantColor ? [image.dominantColor] : []));

	let sourceLink = $derived.by(() => {
		const u = save.originUrl;
		if (!u) return null;
		try {
			const p = new URL(u);
			return p.protocol === 'http:' || p.protocol === 'https:' ? p : null;
		} catch {
			return null;
		}
	});

	let attribution = $derived(save.viewer?.attribution ?? image?.attribution ?? null);
	let hasAttribution = $derived(
		!!attribution && (!!attribution.credit || !!attribution.license || !!attribution.url)
	);

	let savedIn = $derived.by(() => {
		const known = [...collections.items, ...favouriteCollections.items];
		return (save.viewer?.saves ?? []).map((s) => {
			if (s.collectionUri === '') return { uri: '', name: 'Unsorted' };
			const c = known.find((x) => x.uri === s.collectionUri);
			return { uri: s.collectionUri, name: c?.name ?? 'Untitled collection' };
		});
	});

	let createdAt = $derived.by(() => {
		try {
			return new Date(save.createdAt).toLocaleDateString(undefined, {
				year: 'numeric',
				month: 'short',
				day: 'numeric'
			});
		} catch {
			return '';
		}
	});

	// Desktop: animate the outer width while a fixed-width inner panel is clipped, so
	// the central grid reflows smoothly and the panel slides in from the right edge.
	// Mobile: the panel is a full-screen fixed overlay, so slide it in via translateX
	// (matching the left sidebar's offcanvas sheet) without reflowing the grid.
	function slidePanel(node: HTMLElement, { duration = 200 } = {}) {
		if (sidebar.isMobile) {
			return {
				duration,
				easing: linear,
				css: (t: number) => `transform: translateX(${(1 - t) * 100}%)`
			};
		}
		const width = parseFloat(getComputedStyle(node).width) || 352;
		return { duration, easing: linear, css: (t: number) => `width: ${t * width}px` };
	}
</script>

<!-- On mobile the panel overlays the content full-screen (fixed, out of flow) so the
     grid keeps its width; on desktop it's an in-flow flex sibling that pushes the grid. -->
<div
	class={sidebar.isMobile
		? 'fixed inset-y-0 right-0 z-50 w-full overflow-hidden'
		: 'h-full w-[22rem] shrink-0 overflow-hidden'}
	transition:slidePanel
>
	<Sidebar.Root
		side="right"
		collapsible="none"
		class="h-full {sidebar.isMobile ? 'w-full' : 'w-[22rem]'}"
	>
		<Tabs.Root bind:value={tab} class="flex min-h-0 flex-1 flex-col">
			<div class="flex items-center justify-between gap-2 p-3">
				<Tabs.List>
					<Tabs.Trigger value="details">Details</Tabs.Trigger>
					<Tabs.Trigger value="similar">Related</Tabs.Trigger>
				</Tabs.List>
				<Button variant="ghost" size="icon-sm" onclick={onClose} aria-label="Close details">
					<X class="size-4" />
				</Button>
			</div>

			<Tabs.Content
				value="details"
				class="mt-0 flex min-h-0 flex-col gap-5 overflow-y-auto px-4 pt-1 pb-6"
			>
				{#if image}
					<LabeledMedia labels={save.labels} class="flex justify-center">
						<img
							src={image.imageUrl}
							alt={image.alt ?? save.text ?? ''}
							class="max-h-[45vh] w-auto max-w-full object-contain"
							style={image.dominantColor ? `background-color: ${image.dominantColor}` : undefined}
						/>
					</LabeledMedia>
				{/if}

				<div class="flex items-center gap-2">
					<Button variant="secondary" size="sm" class="flex-1" onclick={() => onFindSimilar(save)}>
						<Sparkles class="size-4" />
						Find similar in library
					</Button>
					{#if image}
						<Button
							variant="outline"
							size="icon-sm"
							aria-label="Copy image"
							title="Copy image"
							onclick={() => copyImage(save)}
						>
							<Copy class="size-4" />
						</Button>
					{/if}
					<Button
						variant="outline"
						size="icon-sm"
						aria-label="Copy link"
						title="Copy link"
						onclick={() => copyLink(save)}
					>
						<LinkIcon class="size-4" />
					</Button>
					{#if image}
						<Button
							variant="outline"
							size="icon-sm"
							aria-label="Download image"
							title="Download"
							onclick={() => downloadImage(save)}
						>
							<Download class="size-4" />
						</Button>
					{/if}
				</div>

				{#if palette.length > 0}
					<section class="flex flex-col gap-1.5">
						<h3 class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
							Palette
						</h3>
						<div class="flex h-9 overflow-hidden rounded-md ring-1 ring-border">
							{#each palette as hex (hex)}
								<div class="flex-1" style="background-color: {hex}" title={hex}></div>
							{/each}
						</div>
					</section>
				{/if}

				<section class="flex flex-col gap-1.5">
					<h3 class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
						Saved in
					</h3>
					<div class="flex flex-wrap items-center gap-1.5">
						{#each savedIn as c (c.uri)}
							<Badge variant="secondary" class="font-normal">{c.name}</Badge>
						{/each}
						<CollectionSelector
							item={save}
							variant={sidebar.isMobile ? 'drawer' : 'popover'}
							{onSavesChange}
						>
							{#snippet trigger({ props })}
								<button
									{...props}
									type="button"
									class={cn(
										badgeVariants({ variant: 'outline' }),
										'cursor-pointer gap-1 border-dashed text-muted-foreground hover:bg-muted hover:text-foreground'
									)}
								>
									<Plus />
									Save to collection…
								</button>
							{/snippet}
						</CollectionSelector>
					</div>
				</section>

				{#if save.text}
					<section class="flex flex-col gap-1.5">
						<h3 class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Notes</h3>
						<p class="text-sm whitespace-pre-wrap">{save.text}</p>
					</section>
				{/if}

				{#if sourceLink}
					<section class="flex flex-col gap-1.5">
						<h3 class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
							Source
						</h3>
						<a
							href={sourceLink.href}
							target="_blank"
							rel="noopener noreferrer"
							class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
						>
							<span class="truncate">{sourceLink.hostname}</span>
							<ExternalLink class="size-3.5 shrink-0" />
						</a>
					</section>
				{/if}

				{#if hasAttribution && attribution}
					<section class="flex flex-col gap-1.5">
						<h3 class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
							Attribution
						</h3>
						<div class="flex flex-col gap-1 text-sm text-muted-foreground">
							{#if attribution.credit}<span>Credit: {attribution.credit}</span>{/if}
							{#if attribution.license}<span>License: {attribution.license}</span>{/if}
							{#if attribution.url}
								<a
									href={attribution.url}
									target="_blank"
									rel="noopener noreferrer"
									class="inline-flex items-center gap-1 hover:text-foreground"
								>
									<span class="truncate">Attribution link</span>
									<ExternalLink class="size-3 shrink-0" />
								</a>
							{/if}
						</div>
					</section>
				{/if}

				<section class="flex flex-col gap-1.5">
					<h3 class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Info</h3>
					<dl class="flex flex-col gap-1 text-sm">
						{#if image?.width && image?.height}
							<div class="flex justify-between gap-2">
								<dt class="text-muted-foreground">Dimensions</dt>
								<dd>{image.width} × {image.height}</dd>
							</div>
						{/if}
						<div class="flex justify-between gap-2">
							<dt class="text-muted-foreground">Saved by</dt>
							<dd class="truncate">@{save.author.handle}</dd>
						</div>
						{#if createdAt}
							<div class="flex justify-between gap-2">
								<dt class="text-muted-foreground">Saved on</dt>
								<dd>{createdAt}</dd>
							</div>
						{/if}
						{#if image?.alt}
							<div class="flex flex-col gap-0.5 pt-1">
								<dt class="text-muted-foreground">Alt text</dt>
								<dd class="text-sm">{image.alt}</dd>
							</div>
						{/if}
					</dl>
				</section>
			</Tabs.Content>

			<Tabs.Content value="similar" class="mt-0 min-h-0 flex-1 overflow-hidden">
				<!-- Mount the related grid only while its tab is active: the content div
				     stays in the DOM when hidden, so an always-mounted masonry would
				     measure clientWidth 0 and fetch before it's ever viewed. -->
				{#if tab === 'similar'}
					<SimilarPanel {save} />
				{/if}
			</Tabs.Content>
		</Tabs.Root>
	</Sidebar.Root>
</div>
