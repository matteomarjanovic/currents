<script lang="ts">
	import { setMode, resetMode, userPrefersMode } from 'mode-watcher';
	import * as Select from '$lib/components/ui/select';
	import { cn } from '$lib/utils.js';
	import Sun from '@lucide/svelte/icons/sun';
	import Moon from '@lucide/svelte/icons/moon';
	import Monitor from '@lucide/svelte/icons/monitor';

	let { class: className = '' }: { class?: string } = $props();

	const OPTIONS = [
		{ value: 'light', label: 'Light', Icon: Sun },
		{ value: 'dark', label: 'Dark', Icon: Moon },
		{ value: 'system', label: 'System', Icon: Monitor }
	] as const;

	function onValueChange(value: string) {
		if (value === 'system') resetMode();
		else setMode(value as 'light' | 'dark');
	}

	let current = $derived(
		OPTIONS.find((o) => o.value === (userPrefersMode.current ?? 'system')) ?? OPTIONS[2]
	);
</script>

<Select.Root type="single" value={userPrefersMode.current ?? 'system'} {onValueChange}>
	<Select.Trigger
		class={cn(
			'h-8 w-auto gap-1.5 rounded-full px-3 text-sm text-muted-foreground shadow-none hover:bg-accent',
			className
		)}
		aria-label="Theme: {current.label}"
	>
		<current.Icon class="size-4" />
	</Select.Trigger>
	<Select.Content align="end">
		{#each OPTIONS as { value, label, Icon } (value)}
			<Select.Item {value} {label}>
				<Icon class="size-4" />
				{label}
			</Select.Item>
		{/each}
	</Select.Content>
</Select.Root>
