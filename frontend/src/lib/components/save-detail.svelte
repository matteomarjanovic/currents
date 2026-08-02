<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { fly } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import { apiFetch } from '$lib/api';
	import Logo from '$lib/assets/logo.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Avatar from '$lib/components/ui/avatar';
	import * as Accordion from '$lib/components/ui/accordion';
	import * as Item from '$lib/components/ui/item';
	import { Badge } from '$lib/components/ui/badge';
	import { Spinner } from '$lib/components/ui/spinner';
	import FavouriteToggle from '$lib/components/favourite-toggle.svelte';
	import SupporterBadge from '$lib/components/supporter-badge.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import LabeledMedia from '$lib/components/labeled-media.svelte';
	import SaveImage from '$lib/components/save-image.svelte';
	import MasonryGrid from '$lib/components/masonry-grid.svelte';
	import ReportDialog from '$lib/components/report-dialog.svelte';
	import SaveAttributionDialog from '$lib/components/save-attribution-dialog.svelte';
	import ContentLabelDialog from '$lib/components/content-label-dialog.svelte';
	import { shouldHide, effectiveVisibilityForVals } from '$lib/stores/moderation-prefs.svelte';
	import { openSettings } from '$lib/stores/settings.svelte';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import { isLongImage } from '$lib/image-ratio';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ArrowDown from '@lucide/svelte/icons/arrow-down';
	import ChevronUp from '@lucide/svelte/icons/chevron-up';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import EyeOff from '@lucide/svelte/icons/eye-off';
	import Flag from '@lucide/svelte/icons/flag';
	import Tag from '@lucide/svelte/icons/tag';
	import Star from '@lucide/svelte/icons/star';
	import {
		getImageContent,
		type CollectionView,
		type SaveAttribution,
		type SaveView
	} from '$lib/types';

	interface Props {
		save: SaveView;
		onClose?: () => void;
	}

	let { save, onClose }: Props = $props();
	let hydratedSave = $state<SaveView | null>(null);
	let currentSave = $derived(hydratedSave ?? save);
	let image = $derived(getImageContent(currentSave));
	// Past a few multiples of its width, fitting an image by height leaves an
	// unreadable sliver — those render at full width and scroll instead.
	let long = $derived(isLongImage(image?.width, image?.height));
	let hiddenByPrefs = $derived(shouldHide(currentSave.labels));
	// Render the source only as an http(s) link — guards a "javascript:"/"data:"
	// originUrl (defense in depth; the appview validates the scheme on write).
	let sourceLink = $derived.by(() => {
		const u = currentSave.originUrl;
		if (!u) return null;
		try {
			const p = new URL(u);
			return p.protocol === 'http:' || p.protocol === 'https:' ? p : null;
		} catch {
			return null;
		}
	});
	let attributionDialogOpen = $state(false);
	let reportDialogOpen = $state(false);

	let viewerAttr = $derived(currentSave.viewer?.attribution);
	let originalAttr = $derived(image?.attribution);
	let anySaved = $derived((currentSave.viewer?.saves?.length ?? 0) > 0);
	let canAttribute = $derived(!!auth.user && anySaved);
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
				const res = await apiFetch(`/xrpc/is.currents.feed.getSaves?${params}`);
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

	// When the save arrived here mid-resave (an "optimistic" placeholder created
	// elsewhere, e.g. saving from the masonry grid), poll until the real save
	// record is indexed so the attribution dialog can stop "Waiting for save…".
	$effect(() => {
		const uri = save.uri;
		const pending = currentSave.viewer?.saves?.some((s) => s.saveUri === 'optimistic') ?? false;
		if (!auth.user || !pending) return;

		let cancelled = false;
		void (async () => {
			for (let i = 0; i < 15 && !cancelled; i++) {
				await new Promise((r) => setTimeout(r, 1000));
				if (cancelled) return;
				try {
					const params = new URLSearchParams({ uris: uri });
					const res = await apiFetch(`/xrpc/is.currents.feed.getSaves?${params}`);
					if (!res.ok) continue;
					const data = (await res.json()) as { saves?: SaveView[] };
					const fetched = data.saves?.[0];
					const resolved =
						fetched?.uri === uri &&
						(fetched.viewer?.saves?.length ?? 0) > 0 &&
						!fetched.viewer?.saves?.some((s) => s.saveUri === 'optimistic');
					if (resolved) {
						if (!cancelled) hydratedSave = fetched;
						return;
					}
				} catch (error) {
					console.error('resolve pending save failed', error);
				}
			}
		})();

		return () => {
			cancelled = true;
		};
	});

	// ── Long-image scroll affordance ─────────────────────────────────────────
	// Nothing about a width-fitted long image says "this scrolls", so the pane
	// carries a bottom fade until it's scrolled out. `long` guarantees the image
	// overflows the pane many times over, so "not at the end" is the right initial
	// state — no measuring needed before the first scroll.
	let imagePane: HTMLDivElement | undefined = $state();
	let imagePaneAtEnd = $state(false);

	function onImagePaneScroll(e: Event) {
		const el = e.currentTarget as HTMLDivElement;
		imagePaneAtEnd = el.scrollTop + el.clientHeight >= el.scrollHeight - 1;
	}

	// A new save reuses the pane, so send it back to the top of the image.
	$effect(() => {
		void currentSave.uri;
		imagePane?.scrollTo({ top: 0 });
		imagePaneAtEnd = false;
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

	// ── Floating controls ────────────────────────────────────────────────────
	// The back / scroll-to-top buttons appear once the primary view has scrolled
	// out of sight: the in-flow top bar on mobile, the full-height hero on
	// desktop. Each ref is only laid out on its own breakpoint — the other is
	// `display:none`, so it reads as not-visible and the AND falls through to the
	// active one. Initialising to `true` avoids a flash before the observers run.
	// Viewport-relative, so this works for both the full-page route and the
	// `fixed inset-0` masonry overlay.
	let topControls: HTMLDivElement | undefined = $state();
	let desktopHero: HTMLDivElement | undefined = $state();
	let mobileTopVisible = $state(true);
	let desktopHeroVisible = $state(true);
	let scrolledPastTop = $derived(!mobileTopVisible && !desktopHeroVisible);

	// The scroll-to-top button is fixed (viewport-anchored) but should align with
	// the content's right edge, which sits a classic-scrollbar width further in on
	// platforms that render one (Windows). Measure the scroll container's scrollbar
	// and add it to the offset; with overlay scrollbars this is 0.
	let toTopEl: HTMLDivElement | undefined = $state();
	let scrollbarInset = $state(0);
	function measureScrollbarInset() {
		let p = toTopEl?.parentElement ?? null;
		let w = null;
		while (p) {
			const o = getComputedStyle(p).overflowY;
			if (o === 'auto' || o === 'scroll') {
				w = p.offsetWidth - p.clientWidth;
				break;
			}
			p = p.parentElement;
		}
		w ??= window.innerWidth - document.documentElement.clientWidth;
		// Ratchet: a scroll lock (dialog/drawer) reports 0 while it hides the
		// scrollbar; don't let a resize during that window zero the offset.
		if (w > 0) scrollbarInset = w;
	}
	$effect(() => {
		if (!toTopEl) return;
		measureScrollbarInset();
		window.addEventListener('resize', measureScrollbarInset);
		return () => window.removeEventListener('resize', measureScrollbarInset);
	});

	$effect(() => {
		if (!topControls) return;
		const observer = new IntersectionObserver(([entry]) => {
			mobileTopVisible = entry.isIntersecting;
		});
		observer.observe(topControls);
		return () => observer.disconnect();
	});

	$effect(() => {
		if (!desktopHero) return;
		const observer = new IntersectionObserver(([entry]) => {
			desktopHeroVisible = entry.isIntersecting;
		});
		observer.observe(desktopHero);
		return () => observer.disconnect();
	});

	function scrollToTop() {
		// Scroll the actual container to position 0 (the overlay's `overflow-y-auto`
		// div, or the window for the full-page route). `scrollIntoView` would stop at
		// topControls and skip the container's top padding, landing just short of the top.
		let el: HTMLElement | null = topControls ?? null;
		while (el) {
			const oy = getComputedStyle(el).overflowY;
			if ((oy === 'auto' || oy === 'scroll') && el.scrollHeight > el.clientHeight) {
				el.scrollTo({ top: 0, behavior: 'smooth' });
				return;
			}
			el = el.parentElement;
		}
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}

	let authorName = $derived(currentSave.author.displayName || currentSave.author.handle);

	const related = useInfiniteScroll(async (cursor) => {
		const params = new URLSearchParams({ uri: currentSave.uri, limit: '50' });
		if (cursor) params.set('cursor', cursor);
		const res = await apiFetch(`/xrpc/is.currents.feed.getRelatedSaves?${params}`);
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

	// ── "In other collections" accordion ─────────────────────────────────────
	// The public collections (any user's) that hold the same image, by exact blob
	// CID, ordered by favourite count. Loaded lazily the first time the accordion
	// is opened; "Show more" paginates (an image is rarely in many collections).
	const imageCollections = useInfiniteScroll<CollectionView>(async (cursor) => {
		const params = new URLSearchParams({ uri: currentSave.uri, limit: '50' });
		if (cursor) params.set('cursor', cursor);
		const res = await apiFetch(`/xrpc/is.currents.feed.getImageCollections?${params}`);
		if (!res.ok) return { items: [], cursor: undefined };
		const data = await res.json();
		return { items: data.collections ?? [], cursor: data.cursor };
	});
	let collectionsAccordionValue = $state('');

	$effect(() => {
		void currentSave.uri;
		untrack(() => {
			imageCollections.reset();
			collectionsAccordionValue = '';
		});
	});

	$effect(() => {
		if (
			collectionsAccordionValue === 'collections' &&
			imageCollections.items.length === 0 &&
			imageCollections.hasMore
		) {
			imageCollections.loadMore();
		}
	});

	function collectionHref(c: CollectionView) {
		const rkey = c.uri.split('/').pop() ?? '';
		const handle = c.author?.handle ?? c.uri.split('/')[2];
		return `/profile/${handle}/collection/${rkey}`;
	}

	// First collection preview the viewer is allowed to see, with blur applied per
	// their moderation prefs (mirrors collection-card). null when none are visible.
	function collectionPreview(c: CollectionView) {
		for (const p of c.previews ?? []) {
			const vis = effectiveVisibilityForVals(p.labels);
			if (vis !== 'hide') return { url: p.url, blur: vis === 'blur' };
		}
		return null;
	}

	function handleSavesChange(saves: { collectionUri: string; saveUri: string }[]) {
		const base = hydratedSave ?? save;
		hydratedSave = {
			...base,
			viewer: { ...(base.viewer ?? {}), saves }
		};
	}

	// ── Owner self-label editor (add-only) ───────────────────────────────────
	// Lets the originator add content-warning / AI labels to an already-uploaded
	// save via <ContentLabelDialog>. Owner + non-resave gated; the dialog PUTs the
	// record's `labels` and TAP propagates the change to every copy.
	let isResave = $derived(!!currentSave.resaveOf);
	let canEditLabels = $derived(isOwnSave && !isResave && !!image);
	let labelDialogOpen = $state(false);
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

{#snippet reportButton(extra: string)}
	{#if auth.user && !isOwnSave}
		<Button
			variant="link"
			size="sm"
			class="gap-1 px-0 text-xs text-muted-foreground hover:text-foreground {extra}"
			onclick={() => (reportDialogOpen = true)}
		>
			<Flag class="size-3" />
			Report
		</Button>
	{/if}
{/snippet}

{#snippet info(showReport: boolean)}
	{#if currentSave.text}
		<p class="text-sm whitespace-pre-wrap">{currentSave.text}</p>
	{/if}

	{#if sourceLink}
		<div class="inline-flex items-center gap-1 text-sm text-muted-foreground">
			<span>Source:</span>
			<a
				href={sourceLink.href}
				target="_blank"
				rel="noopener noreferrer"
				class="inline-flex items-center gap-1 hover:text-foreground"
			>
				<span class="truncate">{sourceLink.hostname}</span>
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

	{#if showReport}
		{@render reportButton('self-start')}
	{/if}

	{#if canEditLabels}
		<Button
			variant="link"
			size="sm"
			class="gap-1 self-start px-0 text-xs text-muted-foreground hover:text-foreground"
			onclick={() => (labelDialogOpen = true)}
		>
			<Tag class="size-3" />
			Add content labels
		</Button>
	{/if}
{/snippet}

{#snippet imageCollectionsSection()}
	<Accordion.Root type="single" bind:value={collectionsAccordionValue}>
		<Accordion.Item value="collections">
			<Accordion.Trigger>In other collections</Accordion.Trigger>
			<Accordion.Content>
				{#if imageCollections.loading && imageCollections.items.length === 0}
					<div class="flex justify-center py-3"><Spinner /></div>
				{:else if imageCollections.items.length === 0}
					<p class="px-1 py-1 text-sm text-muted-foreground">Not in any collections yet.</p>
				{:else}
					<div class="max-h-72 overflow-y-auto">
						<Item.Group class="gap-1 pr-1">
							{#each imageCollections.items as c (c.uri)}
								{@const isOwn = !!auth.user && auth.user.did === c.author?.did}
								{@const preview = collectionPreview(c)}
								<Item.Root variant="outline" size="sm" class="relative flex-nowrap">
									<!-- Stretched link: a click anywhere on the row that isn't the username
								     link or the favourite toggle (both lifted with z-10) opens the collection. -->
									<a href={collectionHref(c)} class="absolute inset-0">
										<span class="sr-only">{c.name}</span>
									</a>
									<Item.Media variant="image" class="bg-muted">
										{#if preview}
											<img src={preview.url} alt="" class={preview.blur ? 'blur-md' : ''} />
										{/if}
									</Item.Media>
									<Item.Content class="min-w-0">
										<Item.Title class="w-full truncate">{c.name}</Item.Title>
										<Item.Description class="flex items-center gap-1">
											<a
												href={`/profile/${c.author?.handle}`}
												class="relative z-10 min-w-0 truncate no-underline!"
											>
												{c.author?.displayName || c.author?.handle}
											</a>
											{#if c.author?.supporter}
												<SupporterBadge class="size-3.5 shrink-0" />
											{/if}
											{#if (c.favouriteCount ?? 0) > 0}
												<span aria-hidden="true">·</span>
												<Star class="size-3 shrink-0 fill-current" />
												<span>{c.favouriteCount}</span>
											{/if}
										</Item.Description>
									</Item.Content>
									<Item.Actions class="shrink-0">
										{#if isOwn}
											<Badge variant="secondary">Yours</Badge>
										{:else if auth.user}
											<div class="relative z-10">
												<FavouriteToggle collection={c} labelFrom="never" />
											</div>
										{/if}
									</Item.Actions>
								</Item.Root>
							{/each}
						</Item.Group>
					</div>
					{#if imageCollections.hasMore}
						<div class="mt-2 flex justify-center">
							<Button
								variant="ghost"
								size="sm"
								onclick={() => imageCollections.loadMore()}
								disabled={imageCollections.loading}
							>
								{#if imageCollections.loading}<Spinner />{:else}Show more{/if}
							</Button>
						</div>
					{/if}
				{/if}
			</Accordion.Content>
		</Accordion.Item>
	</Accordion.Root>
{/snippet}

{#snippet saveControl()}
	{#if auth.user && collections.loaded}
		<CollectionSelector
			item={currentSave}
			variant="popover"
			triggerVariant="secondary"
			onSavesChange={handleSavesChange}
		/>
	{:else if auth.checked}
		<Button variant="default" onclick={promptLogin} class="w-full">Save</Button>
	{/if}
{/snippet}

<!-- Mobile Save: a content-width pill rather than the drawer trigger's default
     full-width bar, so the floating back / scroll-to-top buttons sit beside it in
     the same row instead of on top of it. Solid by design — it floats over the
     image, so nothing glassy. -->
{#snippet mobileSaveControl()}
	{#if auth.user && collections.loaded}
		<CollectionSelector item={currentSave} variant="drawer" onSavesChange={handleSavesChange}>
			{#snippet trigger({ props })}
				<Button
					{...props}
					variant={anySaved ? 'secondary' : 'default'}
					size="lg"
					class="rounded-full px-6 shadow-md"
				>
					{anySaved ? 'Saved' : 'Save'}
				</Button>
			{/snippet}
		</CollectionSelector>
	{:else if auth.checked}
		<Button variant="default" size="lg" class="rounded-full px-6 shadow-md" onclick={promptLogin}>
			Save
		</Button>
	{/if}
{/snippet}

{#snippet hiddenState()}
	<div
		class="flex flex-col items-center justify-center gap-3 rounded-lg bg-muted p-8 text-center text-sm text-muted-foreground"
	>
		<EyeOff class="size-6" />
		{#if auth.user}
			<p class="font-medium text-foreground">Hidden by your settings</p>
			<p class="max-w-sm text-xs">
				This image carries a label you've chosen not to see. Adjust your moderation preferences to
				change what's shown.
			</p>
			<button
				type="button"
				onclick={() => openSettings('moderation')}
				class="text-xs font-medium text-foreground underline-offset-2 hover:underline"
			>
				Open settings
			</button>
		{:else}
			<p class="font-medium text-foreground">Hidden content</p>
			<p class="max-w-sm text-xs">
				This image is hidden for logged-out visitors. Log in to manage what you see.
			</p>
			<Button
				variant="link"
				size="sm"
				class="h-auto p-0 text-xs font-medium text-foreground"
				onclick={promptLogin}
			>
				Log in
			</Button>
		{/if}
	</div>
{/snippet}

<div bind:this={desktopHero} class="hidden h-screen md:flex">
	<div class="flex w-1/3 flex-col gap-5 overflow-y-auto border-r border-border p-6 pt-20">
		{@render saveControl()}
		{@render info(true)}
		{@render imageCollectionsSection()}
		<div class="text-md mt-auto flex flex-col items-center gap-2 text-center text-muted-foreground">
			<p>Scroll down to view related images</p>
			<ArrowDown class="size-4" />
		</div>
	</div>
	<!-- Long images fill the pane's width and scroll it; everything else is
	     contained and centred as before. The wrapper is the positioning context for
	     the "more below" fade, which has to sit outside the scrolling element. -->
	<div class="relative w-2/3">
		<!-- The fade is a mask on the pane, not a colored gradient on top of it: this
		     view renders both as a route (page background) and as the pushState overlay
		     (app-muted-wash), and a mask doesn't need to know which. -->
		<div
			bind:this={imagePane}
			onscroll={onImagePaneScroll}
			class="flex h-full justify-center p-6 {long
				? 'items-start overflow-y-auto'
				: 'items-center'} {long && !imagePaneAtEnd ? 'mask-b-from-88% mask-b-to-100%' : ''}"
		>
			{#if hiddenByPrefs}
				{@render hiddenState()}
			{:else if image}
				<LabeledMedia
					labels={currentSave.labels}
					class={long ? 'w-full' : 'flex h-full w-full items-center justify-center'}
				>
					<SaveImage
						{image}
						alt={image.alt ?? currentSave.text ?? ''}
						class={long ? 'w-full' : 'max-h-full max-w-full object-contain'}
						wrapperClass={long ? 'block w-full' : 'flex h-full w-full items-center justify-center'}
						style={image.dominantColor ? `background-color: ${image.dominantColor}` : undefined}
					/>
				</LabeledMedia>
			{:else}
				<div
					class="flex h-full w-full items-center justify-center rounded-lg bg-muted text-sm text-muted-foreground"
				>
					Unsupported content
				</div>
			{/if}
		</div>
		{#if long && !hiddenByPrefs && image}
			<!-- Says out loud what the fade implies. Both clear together once there's
			     nothing left to scroll to. -->
			<div
				class="pointer-events-none absolute inset-x-0 bottom-6 flex justify-center transition-opacity duration-200 {imagePaneAtEnd
					? 'opacity-0'
					: 'opacity-100'}"
			>
				<span
					class="flex items-center gap-1.5 rounded-full bg-primary-foreground/80 px-3 py-1.5 text-xs text-foreground shadow-sm backdrop-blur-sm"
				>
					<ArrowDown class="size-3" />
					Scroll to see the whole image
				</span>
			</div>
		{/if}
	</div>
</div>

<div class="md:hidden">
	<!-- One viewport-tall stage: the image takes whatever height is left over and is
	     centred in it, so a short image sits mid-screen and a long one simply makes
	     the stage taller. `flex-1` grows into free space but never squashes the image
	     (flex items keep min-height:auto), which is what makes both cases work. -->
	<div
		class="flex min-h-dvh flex-col gap-4 p-2"
		style="padding-top: calc(env(safe-area-inset-top) + 1rem)"
	>
		<div bind:this={topControls} class="flex items-center justify-between gap-2">
			<Button variant="ghost" size="icon-sm" onclick={goBack} aria-label="Go back">
				<ArrowLeft class="size-4" />
			</Button>
			{@render reportButton('')}
		</div>
		<div class="flex flex-1 items-center justify-center">
			{#if hiddenByPrefs}
				{@render hiddenState()}
			{:else if image}
				<LabeledMedia labels={currentSave.labels} class="flex justify-center">
					<SaveImage
						{image}
						alt={image.alt ?? currentSave.text ?? ''}
						class={long ? 'w-full' : 'max-h-[65dvh] w-auto max-w-full object-contain'}
						style={`${image.width && image.height ? `aspect-ratio: ${image.width} / ${image.height};` : ''}${image.dominantColor ? ` background-color: ${image.dominantColor};` : ''}`}
					/>
				</LabeledMedia>
			{:else}
				<div
					class="mx-auto flex max-h-[65dvh] w-full items-center justify-center bg-muted text-sm text-muted-foreground"
					style="aspect-ratio: 3 / 4;"
				>
					Unsupported content
				</div>
			{/if}
		</div>
		<div class="flex flex-col gap-4">
			{@render info(false)}
			{@render imageCollectionsSection()}
		</div>
		<!-- Pinned to the bottom of the stage — the bottom of the viewport until the
		     image and its details run out, then it lands under them. The button's own
		     bottom edge keeps the floating back / scroll-to-top offset so the three
		     read as one row. Plain sticky semantics: no scroll listener.
		     `-mt-4` cancels the stage's gap so the landed pill sits closer to the
		     accordion; the padding above it is the scrim's fade, not spacing. -->
		<div
			class="pointer-events-none sticky bottom-0 z-10 -mx-2 -mt-4 flex justify-center pt-8"
			style="padding-bottom: calc(env(safe-area-inset-bottom) + 1rem)"
		>
			<!-- Scrim, so the pill stays legible over whatever it floats above. Painted
			     in the page's own wash and masked out towards the top, which keeps it
			     exact in both themes and in both contexts this view renders in (route
			     background and pushState overlay are the same mix). -->
			<div class="absolute inset-0 app-muted-wash mask-t-from-0% mask-t-to-100%"></div>
			<div class="pointer-events-auto relative">
				{@render mobileSaveControl()}
			</div>
		</div>
	</div>
</div>

<!-- Mobile floating home button: full wordmark, pinned at top center. -->
<a
	href={resolve('/')}
	aria-label="Go to home"
	class="fixed left-1/2 z-50 flex -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-transparent bg-primary-foreground/80 bg-clip-padding px-4 py-2.5 text-foreground shadow-sm backdrop-blur-sm md:hidden"
	style="top: calc(env(safe-area-inset-top) + 2rem)"
>
	<span class="block h-5">
		<Logo />
	</span>
</a>

<!-- Desktop floating controls: back (icon) + home pill, pinned top-left on the
     grid's left border (md:p-6), at the explore top-bar's vertical offset. -->
<div class="fixed top-2 left-6 z-50 hidden items-center gap-2 md:flex">
	<Button
		variant="glass"
		size="icon-lg"
		class="size-11 rounded-full"
		onclick={goBack}
		aria-label="Go back"
	>
		<ArrowLeft class="size-5" />
	</Button>
	<a
		href={resolve('/')}
		aria-label="Go to home"
		class="flex h-11 items-center justify-center rounded-full border border-transparent bg-primary-foreground/80 bg-clip-padding px-4 text-foreground shadow-sm backdrop-blur-sm"
	>
		<span class="block h-5">
			<Logo />
		</span>
	</a>
</div>

<!-- Floating actions: revealed once the primary view scrolls out of sight. -->
{#if scrolledPastTop}
	<div
		class="fixed left-2 z-50 md:hidden"
		style="bottom: calc(env(safe-area-inset-bottom) + 1rem)"
		transition:fly={{ y: 24, duration: 200, easing: cubicOut }}
	>
		<Button variant="glass" size="lg" class="rounded-full" onclick={goBack} aria-label="Go back">
			<ArrowLeft />
			Back
		</Button>
	</div>
	<!-- Aligned with the grid's right border: content padding (p-2 / md:p-6) plus
	     the scroll container's measured scrollbar width. The `100% - 100vw` term
	     cancels root-viewport changes when a scroll lock hides the scrollbar. -->
	<div
		bind:this={toTopEl}
		class="fixed right-[calc(0.5rem+var(--sb)+100%-100vw)] z-50 md:right-[calc(1.5rem+var(--sb)+100%-100vw)]"
		style="bottom: calc(env(safe-area-inset-bottom) + 1rem); --sb: {scrollbarInset}px"
		transition:fly={{ y: 24, duration: 200, easing: cubicOut }}
	>
		<Button
			variant="glass"
			size="icon-lg"
			class="rounded-full md:size-11"
			onclick={scrollToTop}
			aria-label="Scroll to top"
		>
			<ChevronUp class="size-4 md:size-5" />
		</Button>
	</div>
{/if}

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

{#if canEditLabels}
	<ContentLabelDialog
		bind:open={labelDialogOpen}
		save={currentSave}
		onSaved={(added) => {
			const base = hydratedSave ?? save;
			const now = new Date().toISOString();
			hydratedSave = {
				...base,
				labels: [...(base.labels ?? []), ...added.map((v) => ({ src: '', val: v, cts: now }))]
			};
		}}
	/>
{/if}

{#if auth.user && !isOwnSave}
	<ReportDialog bind:open={reportDialogOpen} save={currentSave} />
{/if}
