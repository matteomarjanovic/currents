<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ChevronUp from '@lucide/svelte/icons/chevron-up';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import FlowField from './flow-field.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { loginPrompt } from '$lib/stores/login-prompt.svelte';

	const OPTIONS = [
		{ value: '1', label: 'Personal', noiseIntensity: 0.5 },
		{ value: '0', label: 'General', noiseIntensity: 3 },
		{ value: '-1', label: 'New worlds', noiseIntensity: 7 }
	];
	const DEFAULT_VALUE = '1';

	const selectedValue = $derived.by(() => {
		const raw = page.url.searchParams.get('personalization');
		return OPTIONS.some((o) => o.value === raw) ? raw! : DEFAULT_VALUE;
	});
	const selected = $derived(OPTIONS.find((o) => o.value === selectedValue)!);

	let open = $state(false);

	function selectValue(v: string) {
		if (v === selectedValue) return;
		const url = new URL(page.url);
		url.searchParams.set('personalization', v);
		goto(url, { replaceState: true, keepFocus: true, noScroll: true });
	}
</script>

<DropdownMenu.Root bind:open>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<button
				{...props}
				type="button"
				class="flex cursor-pointer items-center gap-2 rounded-full border-none bg-primary-foreground/80 py-0.5 pr-0.5 pl-3 backdrop-blur-sm transition-transform duration-100 aria-expanded:scale-95"
				aria-label="Adjust personalization"
				onclick={(e: MouseEvent) => {
					if (!auth.user) {
						e.preventDefault();
						e.stopPropagation();
						loginPrompt.open = true;
						return;
					}
					(props.onclick as ((event: MouseEvent) => void) | undefined)?.(e);
				}}
			>
				<ChevronUp
					class="size-4 text-foreground/60 transition-transform duration-200 {open
						? 'rotate-180'
						: ''}"
				/>
				<span class="text-md font-medium whitespace-nowrap text-foreground">
					{selected.label}
				</span>
				<div class="h-11 w-11 overflow-hidden rounded-full">
					<FlowField noiseIntensity={selected.noiseIntensity} lineCount={7} />
				</div>
			</button>
		{/snippet}
	</DropdownMenu.Trigger>

	<DropdownMenu.Content side="top" align="end" sideOffset={8} class="w-44">
		<DropdownMenu.RadioGroup value={selectedValue} onValueChange={selectValue}>
			{#each OPTIONS as option (option.value)}
				<DropdownMenu.RadioItem value={option.value}>
					{option.label}
				</DropdownMenu.RadioItem>
			{/each}
		</DropdownMenu.RadioGroup>
	</DropdownMenu.Content>
</DropdownMenu.Root>
