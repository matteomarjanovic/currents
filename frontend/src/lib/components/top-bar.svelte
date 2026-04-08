<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Slider } from '$lib/components/ui/slider';
	import * as Popover from '$lib/components/ui/popover';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Avatar from '$lib/components/ui/avatar';
	import { personalization } from '$lib/stores/personalization.svelte';
	import { setMode, resetMode, userPrefersMode } from 'mode-watcher';
	import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
	import LogOut from '@lucide/svelte/icons/log-out';
	import UserIcon from '@lucide/svelte/icons/user';
	import Sun from '@lucide/svelte/icons/sun';
	import Moon from '@lucide/svelte/icons/moon';
	import Monitor from '@lucide/svelte/icons/monitor';

	let {
		user
	}: { user: { did: string; handle: string; displayName?: string; avatar?: string } | null } =
		$props();

	let query = $state('');

	function onsubmit(e: Event) {
		e.preventDefault();
		const trimmed = query.trim();
		if (trimmed) {
			goto(resolve('/search/[query]', { query: encodeURIComponent(trimmed) }));
		}
	}

	const personalizationLabels: Record<number, string> = {
		0: 'Off',
		0.25: 'Low',
		0.5: 'Medium',
		0.75: 'High',
		1: 'Max'
	};
</script>

<header
	class="sticky top-0 z-10 flex w-full items-center gap-3 bg-background/95 px-4 py-3 backdrop-blur-sm"
>
	<a href={resolve('/')} class="text-lg font-semibold text-foreground">Currents</a>

	<div class="flex flex-1 items-center justify-center gap-2">
		<form {onsubmit} class="w-full max-w-md">
			<Input type="search" placeholder="Search..." bind:value={query} class="h-12 rounded-full" />
		</form>

		<Popover.Root>
			<Popover.Trigger class="shrink-0 rounded-full data-[slot=popover-trigger]:p-0">
				<Button variant="ghost" size="icon" class="rounded-full" type="button">
					<SlidersHorizontal class="size-4" />
				</Button>
			</Popover.Trigger>
			<Popover.Content
				class="flex w-48 flex-col items-center gap-3 rounded-2xl border bg-popover/90 backdrop-blur-sm"
			>
				<Slider type="single" bind:value={personalization.value} min={0} max={1} step={0.25} />
				<span>Personalization: {personalizationLabels[personalization.value]}</span>
			</Popover.Content>
		</Popover.Root>
	</div>

	<DropdownMenu.Root>
		<DropdownMenu.Trigger class="shrink-0 rounded-full outline-none">
			<Avatar.Root size="default">
				{#if user?.avatar}
					<Avatar.Image src={user.avatar} alt={user.displayName ?? user.handle} />
				{/if}
				<Avatar.Fallback>
					<UserIcon class="size-4" />
				</Avatar.Fallback>
			</Avatar.Root>
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="end" class="w-48">
			<DropdownMenu.Label>
				{#if user?.displayName}
					<div class="text-base text-primary">{user.displayName}</div>
				{/if}
				<div class="font-normal text-muted-foreground">@{user?.handle}</div>
			</DropdownMenu.Label>
			<DropdownMenu.Separator />
			<div class="mx-1.5 my-0.5 flex items-center gap-0.5">
				<button
					onclick={() => setMode('light')}
					title="Light"
					class="flex flex-1 cursor-default items-center justify-center rounded-2xl px-3 py-2 text-sm font-medium transition-colors {userPrefersMode.current ===
					'light'
						? 'bg-foreground/10 text-foreground'
						: 'text-foreground/50 hover:bg-foreground/10 hover:text-foreground'}"
				>
					<Sun class="pointer-events-none size-4 shrink-0" />
				</button>
				<button
					onclick={() => setMode('dark')}
					title="Dark"
					class="flex flex-1 cursor-default items-center justify-center rounded-2xl px-3 py-2 text-sm font-medium transition-colors {userPrefersMode.current ===
					'dark'
						? 'bg-foreground/10 text-foreground'
						: 'text-foreground/50 hover:bg-foreground/10 hover:text-foreground'}"
				>
					<Moon class="pointer-events-none size-4 shrink-0" />
				</button>
				<button
					onclick={() => resetMode()}
					title="System"
					class="flex flex-1 cursor-default items-center justify-center rounded-2xl px-3 py-2 text-sm font-medium transition-colors {userPrefersMode.current ===
					'system'
						? 'bg-foreground/10 text-foreground'
						: 'text-foreground/50 hover:bg-foreground/10 hover:text-foreground'}"
				>
					<Monitor class="pointer-events-none size-4 shrink-0" />
				</button>
			</div>
			<DropdownMenu.Separator />
			<DropdownMenu.Item
				onclick={() => {
					window.location.href = `${PUBLIC_APPVIEW_URL}/oauth/logout`;
				}}
			>
				<LogOut class="size-4" />
				Log out
			</DropdownMenu.Item>
		</DropdownMenu.Content>
	</DropdownMenu.Root>
</header>
