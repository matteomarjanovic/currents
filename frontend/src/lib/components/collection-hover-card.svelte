<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { CollectionView } from '$lib/types';
	import * as HoverCard from '$lib/components/ui/hover-card';
	import { effectiveVisibilityForVals } from '$lib/stores/moderation-prefs.svelte';
	import { bunnyImageUrl } from '$lib/image-url';
	import FavouriteToggle from '$lib/components/favourite-toggle.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { loadFavouriteCollections } from '$lib/stores/favourites.svelte';

	let {
		collection,
		disabled = false,
		open = $bindable(false),
		onOpenChange,
		side = 'right',
		align = 'start',
		showFavourite = false,
		onFavouriteChange,
		href,
		child: trigger
	}: {
		collection: CollectionView;
		disabled?: boolean;
		open?: boolean;
		onOpenChange?: (open: boolean) => void;
		side?: 'top' | 'right' | 'bottom' | 'left';
		align?: 'start' | 'center' | 'end';
		showFavourite?: boolean;
		onFavouriteChange?: (favourited: boolean, favouriteUri?: string) => void;
		href?: string;
		child: Snippet<[{ props: Record<string, unknown> }]>;
	} = $props();
	let previews = $derived(
		(collection.previews ?? [])
			.map((preview) => ({ ...preview, visibility: effectiveVisibilityForVals(preview.labels) }))
			.filter((preview) => preview.visibility !== 'hide')
			.slice(0, 4)
	);
</script>

<HoverCard.Root
	{disabled}
	bind:open
	{onOpenChange}
	openDelay={400}
	closeDelay={150}
	ignoreNonKeyboardFocus
>
	<HoverCard.Trigger onpointerdown={() => (open = false)} oncontextmenu={() => (open = false)}>
		{#snippet child({ props })}
			{@render trigger({ props: { ...props, role: 'link' } })}
		{/snippet}
	</HoverCard.Trigger>
	<HoverCard.Content
		{side}
		{align}
		sideOffset={12}
		class="w-60 p-1 data-closed:hidden data-closed:animate-none"
	>
		<div data-collection-hover-card={collection.uri} class="relative flex flex-col gap-3">
			{#if href}
				<a
					{href}
					aria-label="Open {collection.name}"
					class="absolute inset-0 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-ring"
					onclick={() => (open = false)}
				></a>
			{/if}
			{#if previews.length > 0}
				<div
					class="grid aspect-[4/3] grid-cols-2 grid-rows-2 gap-0.5 overflow-hidden rounded-lg bg-muted"
				>
					{#each [0, 1, 2, 3] as i (i)}
						{@const preview = previews[i]}
						{#if preview}
							<img
								src={bunnyImageUrl(preview.url, { aspectRatio: '1:1', width: 384, quality: 78 })}
								alt=""
								class="h-full w-full object-cover {preview.visibility === 'blur' ? 'blur-md' : ''}"
							/>
						{:else}
							<div class="bg-muted"></div>
						{/if}
					{/each}
				</div>
			{/if}
			<div class="flex items-start gap-2 px-2 pb-2">
				<div class="flex min-w-0 flex-1 flex-col gap-1">
					<p class="min-w-0 font-medium break-words">{collection.name}</p>
					<p class="text-xs text-muted-foreground">
						{collection.saveCount ?? 0}
						{collection.saveCount === 1 ? 'save' : 'saves'}
						{#if collection.sectionCount}
							· {collection.sectionCount} {collection.sectionCount === 1 ? 'section' : 'sections'}
						{/if}
					</p>
					{#if collection.author}
						<p class="truncate text-xs text-muted-foreground">@{collection.author.handle}</p>
					{/if}
				</div>
				{#if showFavourite && auth.user && auth.user.did !== (collection.author?.did ?? collection.uri.split('/')[2])}
					<div class="relative z-10 shrink-0">
						<FavouriteToggle
							{collection}
							labelFrom="never"
							onChange={(favourited, favouriteUri) => {
								onFavouriteChange?.(favourited, favouriteUri);
								if (auth.user) void loadFavouriteCollections(auth.user.did);
							}}
						/>
					</div>
				{/if}
			</div>
		</div>
	</HoverCard.Content>
</HoverCard.Root>
