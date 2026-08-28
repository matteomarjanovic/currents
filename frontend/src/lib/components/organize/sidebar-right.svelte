<script lang="ts">
	import { linear, cubicInOut } from 'svelte/easing';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Badge, badgeVariants } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { cn } from '$lib/utils.js';
	import LabeledMedia from '$lib/components/labeled-media.svelte';
	import SaveImage from '$lib/components/save-image.svelte';
	import SimilarPanel from '$lib/components/organize/similar-panel.svelte';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import ColorMenu from '$lib/components/color-menu.svelte';
	import SaveAltDialog from '$lib/components/save-alt-dialog.svelte';
	import SaveAttributionDialog from '$lib/components/save-attribution-dialog.svelte';
	import ContentLabelDialog from '$lib/components/content-label-dialog.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import { favouriteCollections } from '$lib/stores/favourites.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { getImageContent, type SaveAttribution, type SaveView } from '$lib/types';
	import { copyLink, copyImage, downloadImage, shareLink } from '$lib/save-actions';
	import { isNative } from '$lib/platform';
	import X from '@lucide/svelte/icons/x';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import Plus from '@lucide/svelte/icons/plus';
	import Sparkles from '@lucide/svelte/icons/sparkles';
	import Copy from '@lucide/svelte/icons/copy';
	import LinkIcon from '@lucide/svelte/icons/link';
	import Share2 from '@lucide/svelte/icons/share-2';
	import Download from '@lucide/svelte/icons/download';

	let {
		save,
		onClose,
		onSavesChange,
		onFindSimilar,
		onColorSearch
	}: {
		save: SaveView;
		onClose: () => void;
		onSavesChange?: (saves: { collectionUri: string; saveUri: string }[]) => void;
		onFindSimilar: (save: SaveView) => void;
		onColorSearch?: (hex: string, where: 'explore' | 'library') => void;
	} = $props();

	const sidebar = Sidebar.useSidebar();
	const native = isNative();

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

	// ── Detail editing ───────────────────────────────────────────────────────
	// Organize mode is where saved images get annotated, so the three editors
	// live here as well as in the explore detail panel. Gating matches
	// save-detail.svelte: labels are owner-only and refused on resaves by the
	// server; alt and attribution are edits to the viewer's own record, so a
	// resave can carry its own. Edits are applied to a local overlay because the
	// panel's `save` comes from the page's selection, which nothing refetches.
	let isOwnSave = $derived(auth.user?.did === save.author.did);
	let isResave = $derived(!!save.resaveOf);
	let canEditAlt = $derived(isOwnSave && !!image);
	let canAttribute = $derived(!!auth.user && (save.viewer?.saves?.length ?? 0) > 0);
	let canEditLabels = $derived(isOwnSave && !isResave && !!image);

	let altDialogOpen = $state(false);
	let attributionDialogOpen = $state(false);
	let labelDialogOpen = $state(false);

	let altOverride = $state<string | null>(null);
	let attributionOverride = $state<SaveAttribution | null>(null);
	let addedLabels = $state<string[]>([]);
	// Selecting a different image reuses this component, so clear the overlay.
	let overlaidUri = $state('');
	$effect(() => {
		if (save.uri === overlaidUri) return;
		overlaidUri = save.uri;
		altOverride = null;
		attributionOverride = null;
		addedLabels = [];
	});

	let alt = $derived(altOverride ?? image?.alt ?? '');
	let attribution = $derived(
		attributionOverride ?? save.viewer?.attribution ?? image?.attribution ?? null
	);
	let hasAttribution = $derived(
		!!attribution && (!!attribution.credit || !!attribution.license || !!attribution.url)
	);
	let labelVals = $derived([...(save.labels ?? []).map((l) => l.val), ...addedLabels]);

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
	// Mobile: the panel is a full-screen fixed overlay; match the left sidebar's
	// mobile sheet (fade + short slide from the right, ease-in-out).
	function slidePanel(node: HTMLElement, { duration = 200 } = {}) {
		if (sidebar.isMobile) {
			return {
				duration,
				easing: cubicInOut,
				css: (t: number, u: number) => `opacity: ${t}; transform: translateX(${u * 2.5}rem)`
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
			<div
				class="flex items-center justify-between gap-2 p-3 pt-[calc(env(safe-area-inset-top)+0.75rem)]"
			>
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
						<SaveImage
							{image}
							alt={image.alt ?? save.text ?? ''}
							sizes="22rem"
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
					{#if native}
						<Button
							variant="outline"
							size="icon-sm"
							aria-label="Share link"
							title="Share"
							onclick={() => shareLink(save)}
						>
							<Share2 class="size-4" />
						</Button>
					{:else}
						<Button
							variant="outline"
							size="icon-sm"
							aria-label="Copy link"
							title="Copy link"
							onclick={() => copyLink(save)}
						>
							<LinkIcon class="size-4" />
						</Button>
					{/if}
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
								<ColorMenu
									{hex}
									class="h-full flex-1 cursor-pointer transition-[filter] hover:brightness-95 dark:hover:brightness-110"
									onExplore={(h) => onColorSearch?.(h, 'explore')}
									onLibrary={(h) => onColorSearch?.(h, 'library')}
								/>
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

				{#if hasAttribution || canAttribute}
					<section class="flex flex-col gap-1.5">
						<div class="flex items-center justify-between gap-2">
							<h3 class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
								Attribution
							</h3>
							{#if canAttribute}
								<Button
									variant="ghost"
									size="sm"
									class="-mr-2 h-7 text-xs"
									onclick={() => (attributionDialogOpen = true)}
								>
									{hasAttribution ? 'Edit' : 'Add'}
								</Button>
							{/if}
						</div>
						{#if !hasAttribution}
							<p class="text-sm text-muted-foreground">
								Credit the source. Applies to every collection of yours holding this image.
							</p>
						{/if}
						{#if attribution}
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
						{/if}
					</section>
				{/if}

				{#if canEditLabels || labelVals.length > 0}
					<section class="flex flex-col gap-1.5">
						<div class="flex items-center justify-between gap-2">
							<h3 class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
								Labels
							</h3>
							{#if canEditLabels}
								<Button
									variant="ghost"
									size="sm"
									class="-mr-2 h-7 text-xs"
									onclick={() => (labelDialogOpen = true)}
								>
									Add
								</Button>
							{/if}
						</div>
						{#if labelVals.length > 0}
							<div class="flex flex-wrap gap-1">
								{#each labelVals as val (val)}
									<Badge variant="secondary" class="text-xs">{val}</Badge>
								{/each}
							</div>
						{:else}
							<p class="text-sm text-muted-foreground">
								Flag sensitive or AI-generated content. Labels can be added, not removed.
							</p>
						{/if}
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
					</dl>
				</section>

				{#if canEditAlt || alt}
					<section class="flex flex-col gap-1.5">
						<div class="flex items-center justify-between gap-2">
							<h3 class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
								Alt text
							</h3>
							{#if canEditAlt}
								<Button
									variant="ghost"
									size="sm"
									class="-mr-2 h-7 text-xs"
									onclick={() => (altDialogOpen = true)}
								>
									{alt ? 'Edit' : 'Add'}
								</Button>
							{/if}
						</div>
						<p class="text-sm {alt ? '' : 'text-muted-foreground'}">
							{alt || 'Describe the image for people using a screen reader.'}
						</p>
					</section>
				{/if}
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

{#if canEditAlt}
	<SaveAltDialog bind:open={altDialogOpen} {save} onSaved={(next) => (altOverride = next)} />
{/if}

{#if canAttribute}
	<SaveAttributionDialog
		bind:open={attributionDialogOpen}
		{save}
		onSaved={(attr) => (attributionOverride = attr)}
	/>
{/if}

{#if canEditLabels}
	<ContentLabelDialog
		bind:open={labelDialogOpen}
		{save}
		onSaved={(added) => (addedLabels = [...addedLabels, ...added])}
	/>
{/if}
