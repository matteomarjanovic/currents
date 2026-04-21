<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Avatar from '$lib/components/ui/avatar';
	import { setMode, resetMode, userPrefersMode } from 'mode-watcher';
	import { fade } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import LogOut from '@lucide/svelte/icons/log-out';
	import UserIcon from '@lucide/svelte/icons/user';
	import Sun from '@lucide/svelte/icons/sun';
	import Moon from '@lucide/svelte/icons/moon';
	import Monitor from '@lucide/svelte/icons/monitor';
	import SearchIcon from '@lucide/svelte/icons/search';
	import X from '@lucide/svelte/icons/x';
	import Plus from '@lucide/svelte/icons/plus';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import Logo from '$lib/assets/logo.svelte';
	import CollectionCreateDialog from '$lib/components/collection-create-dialog.svelte';
	import { addCollection } from '$lib/stores/collections.svelte';
	import type { CollectionView } from '$lib/types';

	let {
		user,
		landing = false
	}: {
		user: { did: string; handle: string; displayName?: string; avatar?: string } | null;
		landing?: boolean;
	} = $props();

	let query = $state('');
	let searchOpen = $state(false);
	let createCollectionOpen = $state(false);

	function handleCollectionCreated(collection: CollectionView) {
		addCollection(collection);
		goto(resolve('/(with-navbar)/collection/[uri]', { uri: encodeURIComponent(collection.uri) }));
	}

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
</script>

<header
	class="{landing
		? 'fixed bg-background/0'
		: 'sticky bg-background/95 backdrop-blur-sm'} relative top-0 z-10 flex h-15 w-full items-center gap-3 px-4 py-3"
>
	{#if !searchOpen}
		<a
			transition:fade={{ duration: 250, easing: cubicOut }}
			href={resolve('/')}
			class="h-5 text-lg font-semibold text-foreground"><Logo /></a
		>
	{/if}

	<div
		class="absolute inset-y-0 left-1/2 hidden w-full -translate-x-1/2 items-center md:flex md:max-w-sm lg:max-w-md"
	>
		<form {onsubmit} class="w-full md:max-w-sm lg:max-w-md">
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
	</div>

	{#if !searchOpen}
		<div class="flex-1"></div>
		<Button
			variant="ghost"
			size="icon"
			class="shrink-0 rounded-full md:hidden"
			type="button"
			onclick={() => (searchOpen = true)}
		>
			<SearchIcon class="size-4" />
		</Button>
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
				<DropdownMenu.Trigger class="shrink-0 outline-none">
					{#snippet child({ props })}
						<Button {...props} variant="ghost" size="icon" class="rounded-full" type="button">
							<Plus class="size-5" />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end" class="w-48">
					<DropdownMenu.Item onclick={() => (createCollectionOpen = true)}>
						<FolderPlus class="size-4" />
						Create collection
					</DropdownMenu.Item>
					<DropdownMenu.Item onclick={() => goto(resolve('/(with-navbar)/upload'))}>
						<ImagePlus class="size-4" />
						Upload images
					</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
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
					<DropdownMenu.Item
						onclick={() =>
							goto(resolve('/(with-navbar)/profile/[handle]', { handle: user.handle }))}
					>
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

{#if user}
	<CollectionCreateDialog bind:open={createCollectionOpen} onCreated={handleCollectionCreated} />
{/if}
