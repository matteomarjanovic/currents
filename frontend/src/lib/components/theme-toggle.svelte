<script lang="ts">
	import { setMode, resetMode, userPrefersMode } from 'mode-watcher';
	import * as Select from '$lib/components/ui/select';

	const LABELS = { light: 'Light', dark: 'Dark', system: 'System' } as const;

	function onValueChange(value: string) {
		if (value === 'system') resetMode();
		else setMode(value as 'light' | 'dark');
	}
</script>

<Select.Root type="single" value={userPrefersMode.current ?? 'system'} {onValueChange}>
	<Select.Trigger
		class="h-8 w-auto gap-1.5 rounded-full px-3 text-sm text-muted-foreground shadow-none hover:bg-accent"
	>
		{LABELS[userPrefersMode.current ?? 'system']}
	</Select.Trigger>
	<Select.Content align="end">
		{#each Object.entries(LABELS) as [value, label] (value)}
			<Select.Item {value} {label}>{label}</Select.Item>
		{/each}
	</Select.Content>
</Select.Root>
