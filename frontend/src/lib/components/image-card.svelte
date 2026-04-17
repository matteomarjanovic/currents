<script lang="ts">
	import { pushState } from '$app/navigation';
	import { getImageContent, type SaveView } from '$lib/types';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { Button } from '$lib/components/ui/button';
	import CollectionSelector from '$lib/components/collection-selector.svelte';

	interface Props {
		item: SaveView;
	}

	let { item }: Props = $props();

	let dropdownOpen = $state(false);
	let href = $derived(`/save/${encodeURIComponent(item.uri)}`);
	let image = $derived(getImageContent(item));

	function handleClick(e: MouseEvent) {
		// Let the browser handle modified clicks (open in new tab, etc.)
		if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
		e.preventDefault();
		pushState(href, { save: $state.snapshot(item) });
	}
</script>

<div
	class="group relative overflow-hidden rounded-lg"
	style={image?.dominantColor ? `background-color: ${image.dominantColor}` : undefined}
>
	<a {href} class="block" onclick={handleClick}>
		{#if image}
			<img
				src={image.imageUrl}
				alt={item.text ?? ''}
				loading="lazy"
				class="w-full"
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
	</a>
	{#if auth.user && collections.loaded}
		<div
			class="pointer-events-none absolute inset-0 hidden flex-col justify-end bg-black/20 p-2 transition-opacity duration-300 md:flex {dropdownOpen
				? 'opacity-100'
				: 'opacity-0 group-hover:opacity-100'}"
		>
			<div
				class="pointer-events-auto transition-transform duration-300 {dropdownOpen
					? 'translate-y-0'
					: 'translate-y-2 group-hover:translate-y-0'}"
			>
				<CollectionSelector {item} variant="popover" onOpenChange={(o) => (dropdownOpen = o)} />
			</div>
		</div>
	{:else if auth.checked}
		<div
			class="pointer-events-none absolute inset-0 flex flex-col justify-end bg-black/20 p-2 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
		>
			<div
				class="pointer-events-auto flex translate-y-2 items-center justify-end gap-1.5 transition-transform duration-300 group-hover:translate-y-0"
			>
				<Button size="sm" variant="default" onclick={promptLogin}>Save</Button>
			</div>
		</div>
	{/if}
</div>
