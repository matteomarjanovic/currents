<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Slider } from '$lib/components/ui/slider';
	import * as Popover from '$lib/components/ui/popover';
	import { personalization } from '$lib/stores/personalization.svelte';
	import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
	import CircleUser from '@lucide/svelte/icons/circle-user';

	let query = $state('');

	function onsubmit(e: Event) {
		e.preventDefault();
		const trimmed = query.trim();
		if (trimmed) {
			goto(`/search/${encodeURIComponent(trimmed)}`);
		}
	}
</script>

<header class="flex items-center gap-3 border-b border-border px-4 py-3">
	<a href="/" class="text-lg font-semibold text-foreground">Currents</a>

	<div class="flex flex-1 items-center justify-center gap-2">
		<form {onsubmit} class="w-full max-w-md">
			<Input type="search" placeholder="Search..." bind:value={query} />
		</form>

		<Popover.Root>
			<Popover.Trigger class="shrink-0 rounded-full [&[data-slot=popover-trigger]]:p-0">
				<Button variant="ghost" size="icon" class="rounded-full" type="button">
					<SlidersHorizontal class="size-4" />
				</Button>
			</Popover.Trigger>
			<Popover.Content class="w-48">
				<Slider type="single" bind:value={personalization.value} min={0} max={1} step={0.25} />
			</Popover.Content>
		</Popover.Root>
	</div>

	<Button variant="ghost" size="icon" class="shrink-0 rounded-full">
		<CircleUser class="size-5" />
	</Button>
</header>
