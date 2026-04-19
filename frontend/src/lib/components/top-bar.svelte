<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Slider } from '$lib/components/ui/slider';
	import * as Popover from '$lib/components/ui/popover';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Avatar from '$lib/components/ui/avatar';
	import { personalization } from '$lib/stores/personalization.svelte';
	import { setMode, resetMode, userPrefersMode } from 'mode-watcher';
	import { fade } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
	import LogOut from '@lucide/svelte/icons/log-out';
	import UserIcon from '@lucide/svelte/icons/user';
	import Sun from '@lucide/svelte/icons/sun';
	import Moon from '@lucide/svelte/icons/moon';
	import Monitor from '@lucide/svelte/icons/monitor';
	import SearchIcon from '@lucide/svelte/icons/search';
	import X from '@lucide/svelte/icons/x';
	import Logo from '$lib/assets/logo.svelte';

	let {
		user,
		landing = false
	}: {
		user: { did: string; handle: string; displayName?: string; avatar?: string } | null;
		landing?: boolean;
	} = $props();

	let query = $state('');
	let searchOpen = $state(false);
	let isSearchPage = $derived(
		page.url.pathname === '/search' || page.url.pathname.startsWith('/search/')
	);

	$effect(() => {
		if (page.url.pathname === '/explore' || page.url.pathname === '/') query = '';
		else if (page.params.query) query = page.params.query;
	});

	function onsubmit(e: Event) {
		e.preventDefault();
		const trimmed = query.trim();
		if (trimmed) {
			goto(resolve('/(with-navbar)/search/[query]', { query: encodeURIComponent(trimmed) }));
		}
	}

	const personalizationLabels: Record<number, string> = {
		[-1]: 'Serendipity Max',
		[-0.75]: 'Serendipity High',
		[-0.5]: 'Serendipity',
		[-0.25]: 'Serendipity Low',
		0: 'Off',
		0.25: 'Low',
		0.5: 'Medium',
		0.75: 'High',
		1: 'Max'
	};
</script>

<header
	class="{landing
		? 'fixed bg-background/0'
		: 'sticky bg-background/95 backdrop-blur-sm'} top-0 z-10 flex h-15 w-full items-center gap-3 px-4 py-3"
>
	{#if !searchOpen}
		<a
			transition:fade={{ duration: 250, easing: cubicOut }}
			href={resolve('/')}
			class="h-5 text-lg font-semibold text-foreground"><Logo /></a
		>
	{/if}

	<div class="hidden flex-1 items-center justify-center gap-2 md:flex">
		<form {onsubmit} class="w-full max-w-md">
			<Input
				type="search"
				placeholder="Search images..."
				bind:value={query}
				autocorrect="off"
				autocapitalize="off"
				autocomplete="off"
				spellcheck={false}
				class="{landing
					? 'bg-accent/50 backdrop-blur-sm placeholder:text-white/70'
					: ''} h-11 rounded-full"
			/>
		</form>

		{#if user && !isSearchPage}
			<Popover.Root>
				<Popover.Trigger class="shrink-0 rounded-full data-[slot=popover-trigger]:p-0">
					<Button variant="ghost" size="icon" class="rounded-full" type="button">
						<SlidersHorizontal class="size-4" />
					</Button>
				</Popover.Trigger>
				<Popover.Content
					class="flex w-48 flex-col items-center gap-3 rounded-2xl border bg-popover/90 backdrop-blur-sm"
				>
					<Slider type="single" bind:value={personalization.value} min={-1} max={1} step={0.25} />
					<span>Feed: {personalizationLabels[personalization.value]}</span>
				</Popover.Content>
			</Popover.Root>
		{:else}
			<!-- <a href="/login">
				<Button variant="ghost" size="sm" class="shrink-0 rounded-full" type="button">
					Login to personalize
				</Button>
			</a> -->
		{/if}
	</div>

	{#if !searchOpen}
		<div class="flex-1 md:hidden"></div>
		<Button
			variant="ghost"
			size="icon"
			class="shrink-0 rounded-full md:hidden"
			type="button"
			onclick={() => (searchOpen = true)}
		>
			<SearchIcon class="size-4" />
		</Button>
		{#if user && !isSearchPage}
			<Popover.Root>
				<Popover.Trigger class="shrink-0 rounded-full data-[slot=popover-trigger]:p-0  md:hidden">
					<Button variant="ghost" size="icon" class="rounded-full" type="button">
						<SlidersHorizontal class="size-4" />
					</Button>
				</Popover.Trigger>
				<Popover.Content
					class="flex w-48 flex-col items-center gap-3 rounded-2xl border bg-popover/90 backdrop-blur-sm"
				>
					<Slider type="single" bind:value={personalization.value} min={-1} max={1} step={0.25} />
					<span>Feed: {personalizationLabels[personalization.value]}</span>
				</Popover.Content>
			</Popover.Root>
		{/if}
	{/if}

	{#if searchOpen}
		<div
			transition:fade={{ duration: 250, easing: cubicOut }}
			class="absolute inset-0 flex items-center gap-2 px-4 md:hidden"
		>
			<form {onsubmit} class="flex-1">
				<Input
					type="search"
					placeholder="Search images..."
					bind:value={query}
					autofocus
					autocorrect="off"
					autocapitalize="off"
					autocomplete="off"
					spellcheck={false}
					class="{landing
						? 'bg-accent/50 backdrop-blur-sm placeholder:text-white/70'
						: ''} h-11 rounded-full"
				/>
			</form>
			<Button
				variant="outline"
				size="icon"
				class="shrink-0 rounded-full"
				onclick={() => (searchOpen = false)}
			>
				<X class="size-4" />
			</Button>
		</div>
	{/if}

	{#if !searchOpen}
		{#if user}
			<DropdownMenu.Root>
				<DropdownMenu.Trigger class="shrink-0 rounded-full outline-none">
					<Avatar.Root size="default">
						{#if user.avatar}
							<Avatar.Image src={user.avatar} alt={user.displayName ?? user.handle} />
						{/if}
						<Avatar.Fallback>
							<UserIcon class="size-4" />
						</Avatar.Fallback>
					</Avatar.Root>
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end" class="w-48">
					<DropdownMenu.Label>
						{#if user.displayName}
							<div class="text-base text-primary">{user.displayName}</div>
						{/if}
						<div class="font-normal text-muted-foreground">@{user.handle}</div>
					</DropdownMenu.Label>
					<DropdownMenu.Separator />
					<DropdownMenu.Item onclick={() => goto(`/profile/${user.handle}`)}>
						<UserIcon class="size-4" />
						Profile
					</DropdownMenu.Item>
					<DropdownMenu.Item
						onclick={() => {
							window.location.href = `${PUBLIC_APPVIEW_URL}/oauth/logout`;
						}}
					>
						<LogOut class="size-4" />
						Log out
					</DropdownMenu.Item>
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
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		{:else}
			<a href={resolve('/login')}>
				<Button variant="default" size="lg" class="shrink-0 rounded-full px-5">Log in</Button>
			</a>
		{/if}
	{/if}
</header>
