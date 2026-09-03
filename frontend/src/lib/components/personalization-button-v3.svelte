<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ChevronUp from '@lucide/svelte/icons/chevron-up';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Button } from '$lib/components/ui/button';
	import FlowField from './flow-field.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { loginPrompt } from '$lib/stores/login-prompt.svelte';
	import { FEED_LEVELS, findFeedLevel } from '$lib/feed-levels';
	import { blockImageNavigation } from '$lib/image-navigation-guard';

	// The active level comes from the route; default to general if the slug is unknown.
	const selected = $derived(findFeedLevel(page.params.level) ?? FEED_LEVELS[1]);

	let open = $state(false);

	function preventTouchClickThrough(event: TouchEvent) {
		// Bits UI opens this menu from pointerup. Android Chrome can still emit a
		// compatibility click afterwards, then re-hit-test it against the image grid.
		// Cancelling touchstart leaves the pointer events intact but suppresses that click.
		blockImageNavigation();
		event.preventDefault();
	}

	function selectLevel(slug: string) {
		blockImageNavigation();
		const level = findFeedLevel(slug)!;
		// General is the only feed open to logged-out visitors; the rest need auth.
		if (!auth.user && level.value !== 0) {
			loginPrompt.open = true;
			return;
		}
		if (slug === selected.slug) return;
		goto(`/explore/${slug}`, { replaceState: true, keepFocus: true, noScroll: true });
	}
</script>

<DropdownMenu.Root bind:open>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Button
				{...props}
				variant="glass"
				class="h-auto cursor-pointer gap-2 rounded-full p-0.5 transition-transform duration-100 aria-expanded:scale-95 md:pl-3"
				aria-label="Adjust personalization"
				onpointerdowncapture={blockImageNavigation}
				ontouchstart={preventTouchClickThrough}
			>
				<ChevronUp
					class="hidden size-4 text-foreground/60 transition-transform duration-200 md:block {open
						? 'rotate-180'
						: ''}"
				/>
				<span class="text-md hidden font-medium whitespace-nowrap text-foreground md:inline">
					{selected.label}
				</span>
				<div class="h-11 w-11 overflow-hidden rounded-full">
					<FlowField noiseIntensity={selected.noiseIntensity} lineCount={7} />
				</div>
			</Button>
		{/snippet}
	</DropdownMenu.Trigger>

	<DropdownMenu.Content side="top" align="end" sideOffset={8} class="w-44">
		<DropdownMenu.RadioGroup value={selected.slug} onValueChange={selectLevel}>
			{#each FEED_LEVELS as level (level.slug)}
				<DropdownMenu.RadioItem value={level.slug}>
					{level.label}
				</DropdownMenu.RadioItem>
			{/each}
		</DropdownMenu.RadioGroup>
	</DropdownMenu.Content>
</DropdownMenu.Root>
