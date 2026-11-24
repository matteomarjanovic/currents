<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';

	let {
		currentIndex,
		canGoForward,
		onVisit
	}: {
		currentIndex: number;
		canGoForward: boolean;
		onVisit: (index: number) => void;
	} = $props();
</script>

<div
	data-mobile-related-navigation
	class="absolute bottom-[calc(env(safe-area-inset-bottom)+0.75rem)] left-3 z-30 flex h-[58px] items-center gap-1 rounded-full border border-border bg-background/95 p-1 shadow-lg backdrop-blur-xl"
	role="group"
	aria-label="Related navigation"
>
	<Button
		variant="ghost"
		class="size-12 rounded-full"
		disabled={currentIndex === 0}
		onclick={() => onVisit(currentIndex - 1)}
		aria-label="Back in related images"><ArrowLeft class="size-5" /></Button
	>
	<Button
		variant="ghost"
		class="size-12 rounded-full"
		disabled={!canGoForward}
		onclick={() => onVisit(currentIndex + 1)}
		aria-label="Forward in related images"><ArrowRight class="size-5" /></Button
	>
</div>
