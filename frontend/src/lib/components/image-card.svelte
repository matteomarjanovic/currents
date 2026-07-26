<script lang="ts">
	import { pushState } from '$app/navigation';
	import { getImageContent, type SaveView } from '$lib/types';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { Button } from '$lib/components/ui/button';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import LabeledMedia from '$lib/components/labeled-media.svelte';
	import SaveImage from '$lib/components/save-image.svelte';

	interface Props {
		item: SaveView;
		// When false, the image isn't a link to the detail view (clicking does nothing);
		// the hover overlay / collection selector still work. Defaults to true.
		linkToDetail?: boolean;
		// The hover overlay needs a pointer; opt into an always-visible Save button
		// (bottom-right, opens the collection drawer) on mobile viewports.
		mobileSave?: boolean;
	}

	let { item, linkToDetail = true, mobileSave = false }: Props = $props();

	let dropdownOpen = $state(false);
	let href = $derived.by(() => {
		const rkey = item.uri.split('/').pop() ?? '';
		return `/profile/${item.author.handle}/save/${rkey}`;
	});
	let image = $derived(getImageContent(item));

	function handleClick(e: MouseEvent) {
		// Let the browser handle modified clicks (open in new tab, etc.)
		if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
		e.preventDefault();
		pushState(href, { save: $state.snapshot(item) });
	}

	// Keep viewer save state on the item in sync so the snapshot pushed to the
	// detail view reflects saves made here (drives the "Add attribution" button).
	function handleSavesChange(saves: { collectionUri: string; saveUri: string }[]) {
		item.viewer = { ...(item.viewer ?? {}), saves };
	}

	let anySaved = $derived((item.viewer?.saves ?? []).length > 0);
</script>

{#snippet media()}
	{#if image}
		<SaveImage
			{image}
			alt={image.alt ?? item.text ?? ''}
			class="w-full"
			wrapperClass="block w-full"
			style={image.width && image.height
				? `aspect-ratio: ${image.width} / ${image.height}`
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
	style={image?.dominantColor ? `background-color: ${image.dominantColor}` : undefined}
>
	<LabeledMedia labels={item.labels}>
		{#if linkToDetail}
			<a {href} class="block" onclick={handleClick}>{@render media()}</a>
		{:else}
			<div class="block">{@render media()}</div>
		{/if}
		{#snippet overlay()}
			{#if auth.user && collections.loaded}
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
