<script lang="ts">
	import SupporterBadge from '$lib/components/supporter-badge.svelte';
	import Check from '@lucide/svelte/icons/check';
	import { cn } from '$lib/utils.js';

	// The supporter-tier perk list, shared by the support page and the
	// upgrade dialog so they never drift apart.
	let { class: className = '' }: { class?: string } = $props();
</script>

<ul class={cn('flex flex-col gap-2.5', className)}>
	<li class="flex items-start gap-2.5">
		<Check class="mt-1 size-4 shrink-0 text-foreground" />
		<span>Semantic search in your library</span>
	</li>
	<li class="flex items-start gap-2.5">
		<Check class="mt-1 size-4 shrink-0 text-foreground" />
		<span>Image search in your library ("find similar")</span>
	</li>
	<li class="flex items-start gap-2.5">
		<Check class="mt-1 size-4 shrink-0 text-foreground" />
		<span>Both filterable by collection</span>
	</li>
	<li class="flex items-start gap-2.5">
		<Check class="mt-1 size-4 shrink-0 text-foreground" />
		<span>The <SupporterBadge class="mx-0.5 mb-0.5 size-4" /> supporter badge</span>
	</li>
	<li class="flex items-start gap-2.5">
		<Check class="mt-1 size-4 shrink-0 text-foreground" />
		<span>Much more to come</span>
	</li>
</ul>
