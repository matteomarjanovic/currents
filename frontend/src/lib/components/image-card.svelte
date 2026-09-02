<script lang="ts">
	import { pushState } from '$app/navigation';
	import { getImageContent, type SaveView } from '$lib/types';
	import { isCropped, tileRatio } from '$lib/image-ratio';
	import { longpress } from '$lib/long-press';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { Button } from '$lib/components/ui/button';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import LabeledMedia from '$lib/components/labeled-media.svelte';
	import SaveImage from '$lib/components/save-image.svelte';
	import MobileQuickActionsDrawer from '$lib/components/mobile-quick-actions-drawer.svelte';
	import ImageActionMenu from '$lib/components/image-action-menu.svelte';

	interface Props {
		item: SaveView;
		// When false, the image isn't a link to the detail view (clicking does nothing);
		// the hover overlay / collection selector still work. Defaults to true.
		linkToDetail?: boolean;
		// The hover overlay needs a pointer; opt into an always-visible Save button
		// (bottom-right, opens the collection drawer) on mobile viewports.
		mobileSave?: boolean;
		// Opt into touch actions: holding the tile opens Quick actions, whose save
		// controls include Quick Save and the full collection picker.
		longPressSave?: boolean;
		// Pre-mount desktop controls before the user hovers so their first reveal can animate.
		preloadControls?: boolean;
		// Called just before the detail view opens, so the grid can record the run of
		// images this tile came from (see $lib/save-sequence).
		onOpen?: () => void;
	}

	let {
		item,
		linkToDetail = true,
		mobileSave = false,
		longPressSave = false,
		preloadControls = false,
		onOpen
	}: Props = $props();

	let dropdownOpen = $state(false);
	let desktopControlsMounted = $state(false);
	let quickActionsOpen = $state(false);
	let quickActionsMounted = $state(false);
	let suppressNextClick = false;
	let href = $derived.by(() => {
		const rkey = item.uri.split('/').pop() ?? '';
		return `/profile/${item.author.handle}/save/${rkey}`;
	});
	let image = $derived(getImageContent(item));
	// A very tall image renders as a top crop at the clamped ratio; the <img> keeps
	// its own dimensions but is covered into the shorter box (see $lib/image-ratio).
	let ratio = $derived(tileRatio(image?.width, image?.height));
	let cropped = $derived(isCropped(image?.width, image?.height));
	// The tile's own box, so it stands in its dominant color at the right size while
	// <LabeledMedia> holds a labeled image back (see prefsPending) — the height would
	// otherwise come from the <img> alone and collapse to nothing. Same ratio the
	// image renders at, and only where the image carries dimensions, so a tile with
	// unknown ones keeps sizing itself rather than being cropped to a guessed 3:4.
	let tileStyle = $derived(
		[
			image?.dominantColor ? `background-color: ${image.dominantColor}` : '',
			image?.width && image?.height ? `aspect-ratio: ${ratio.width} / ${ratio.height}` : ''
		]
			.filter(Boolean)
			.join('; ') || undefined
	);

	function handleLongPress() {
		if (!auth.user) {
			promptLogin();
			return;
		}
		if (!collections.loaded) return;
		suppressNextClick = true;
		quickActionsMounted = true;
		quickActionsOpen = true;
	}

	function handleClick(e: MouseEvent) {
		if (suppressNextClick) {
			suppressNextClick = false;
			e.preventDefault();
			return;
		}
		// Let the browser handle modified clicks (open in new tab, etc.)
		if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
		e.preventDefault();
		onOpen?.();
		pushState(href, { save: $state.snapshot(item) });
	}

	function prepareDesktopControls(node: HTMLElement) {
		function onPointerEnter(event: PointerEvent) {
			if (event.pointerType === 'mouse') desktopControlsMounted = true;
		}
		function onFocusIn() {
			desktopControlsMounted = true;
		}
		node.addEventListener('pointerenter', onPointerEnter);
		node.addEventListener('focusin', onFocusIn);
		return {
			destroy() {
				node.removeEventListener('pointerenter', onPointerEnter);
				node.removeEventListener('focusin', onFocusIn);
			}
		};
	}

	// Keep viewer save state on the item in sync so the snapshot pushed to the
	// detail view reflects saves made here (drives the "Add attribution" button).
	function handleSavesChange(saves: { collectionUri: string; saveUri: string }[]) {
		item.viewer = { ...(item.viewer ?? {}), saves };
	}

	let anySaved = $derived((item.viewer?.saves ?? []).length > 0);

	$effect(() => {
		if (preloadControls) desktopControlsMounted = true;
	});
</script>

{#snippet media()}
	{#if image}
		<SaveImage
			{image}
			variant="grid"
			alt={image.alt ?? item.text ?? ''}
			class="w-full {cropped ? 'object-cover object-top' : ''}"
			wrapperClass="block w-full"
			overlayFit={cropped ? 'object-cover object-top' : 'object-contain'}
			style={image.width && image.height
				? `aspect-ratio: ${ratio.width} / ${ratio.height}`
				: undefined}
		/>
	{:else}
		<div
			class="flex items-center justify-center bg-muted text-sm text-muted-foreground"
			style="aspect-ratio: 3 / 4;"
		>
			Unsupported content
		</div>
	{/if}
{/snippet}

<div
	class="group relative overflow-hidden rounded-lg"
	style={tileStyle}
	use:longpress={{ enabled: longPressSave, onLongPress: handleLongPress }}
	use:prepareDesktopControls
>
	<ImageActionMenu {item} variant="context">
		<LabeledMedia labels={item.labels}>
			{#if linkToDetail}
				<a {href} class="block" draggable={false} onclick={handleClick}>
					{@render media()}
				</a>
			{:else}
				<div class="block">{@render media()}</div>
			{/if}
			{#snippet overlay()}
				{#if auth.user && collections.loaded && desktopControlsMounted}
					<div
						class="pointer-events-none absolute inset-0 hidden flex-col justify-end bg-black/20 p-2 transition-opacity duration-300 md:flex {dropdownOpen
							? 'opacity-100'
							: 'opacity-0 group-hover:opacity-100'}"
					>
						<div
							class="transition-transform duration-300 {dropdownOpen
								? 'pointer-events-auto translate-y-0'
								: 'pointer-events-none translate-y-2 group-hover:pointer-events-auto group-hover:translate-y-0'}"
						>
							<CollectionSelector
								{item}
								variant="popover"
								onOpenChange={(o) => (dropdownOpen = o)}
								onSavesChange={handleSavesChange}
							/>
						</div>
					</div>
				{:else if auth.checked}
					<div
						class="pointer-events-none absolute inset-0 flex flex-col justify-end bg-black/20 p-2 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
					>
						<div
							class="pointer-events-none flex translate-y-2 items-center justify-end gap-1.5 transition-transform duration-300 group-hover:pointer-events-auto group-hover:translate-y-0"
						>
							<Button size="sm" variant="default" onclick={promptLogin}>Save</Button>
						</div>
					</div>
				{/if}
			{/snippet}
		</LabeledMedia>
	</ImageActionMenu>
	{#if longPressSave && auth.user && collections.loaded && quickActionsMounted}
		<MobileQuickActionsDrawer
			{item}
			bind:open={quickActionsOpen}
			onSavesChange={handleSavesChange}
		/>
	{/if}
	{#if mobileSave && auth.user && collections.loaded}
		<div class="absolute right-1.5 bottom-1.5 md:hidden">
			<CollectionSelector {item} variant="drawer" onSavesChange={handleSavesChange}>
				{#snippet trigger({ props })}
					<Button
						{...props}
						variant={anySaved ? 'secondary' : 'default'}
						size="sm"
						class="rounded-full shadow-md"
					>
						{anySaved ? 'Saved' : 'Save'}
					</Button>
				{/snippet}
			</CollectionSelector>
		</div>
	{/if}
</div>
