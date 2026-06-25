<script lang="ts">
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { apiFetch } from '$lib/api';
	import { clearAuthToken } from '$lib/auth-storage';
	import { isNative } from '$lib/platform';
	import { auth } from '$lib/stores/auth.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as InputGroup from '$lib/components/ui/input-group';
	import * as Select from '$lib/components/ui/select';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Avatar from '$lib/components/ui/avatar';
	import { setMode, resetMode, userPrefersMode } from 'mode-watcher';
	import { fade } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import LogOut from '@lucide/svelte/icons/log-out';
	import UserIcon from '@lucide/svelte/icons/user';
	import Download from '@lucide/svelte/icons/download';
	import Sun from '@lucide/svelte/icons/sun';
	import Moon from '@lucide/svelte/icons/moon';
	import Monitor from '@lucide/svelte/icons/monitor';
	import SearchIcon from '@lucide/svelte/icons/search';
	import X from '@lucide/svelte/icons/x';
	import Plus from '@lucide/svelte/icons/plus';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import Puzzle from '@lucide/svelte/icons/puzzle';
	import Settings from '@lucide/svelte/icons/settings';
	import Bell from '@lucide/svelte/icons/bell';
	import Logo from '$lib/assets/logo.svelte';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import CollectionCreateDialog from '$lib/components/collection-create-dialog.svelte';
	import BrowserExtensionDialog from '$lib/components/browser-extension-dialog.svelte';
	import NotificationsDialog from '$lib/components/notifications-dialog.svelte';
	import { addCollection } from '$lib/stores/collections.svelte';
	import { notifications, refreshNotifications } from '$lib/stores/notifications.svelte';
	import { social, refreshSocial } from '$lib/stores/social.svelte';
	import {
		features,
		loadSeenFeatures,
		markFeatureSeen,
		isFeatureSeen,
		hasUnseenAnnouncement,
		FEATURE_PINTEREST_IMPORT,
		FEATURE_BLUESKY_IMPORT
	} from '$lib/stores/features.svelte';
	import { loadModerationPrefs, modPrefsLoaded } from '$lib/stores/moderation-prefs.svelte';
	import { onMount } from 'svelte';
	import { detectBrowser } from '$lib/browser';
	import type { CollectionView } from '$lib/types';

	let {
		user,
		landing = false
	}: {
		user: { did: string; handle: string; displayName?: string; avatar?: string } | null;
		landing?: boolean;
	} = $props();

	const SEARCH_TYPES = [
		{ value: 'saves', label: 'Images' },
		{ value: 'collections', label: 'Collections' },
		{ value: 'users', label: 'Users' }
	] as const;
	type SearchType = (typeof SEARCH_TYPES)[number]['value'];

	let query = $state('');
	let searchType = $state<SearchType>('saves');
	let searchLabel = $derived(SEARCH_TYPES.find((t) => t.value === searchType)?.label ?? 'Images');
	let searchOpen = $state(false);
	let createCollectionOpen = $state(false);
	let browserExtensionDialogOpen = $state(false);
	let notificationsOpen = $state(false);

	// Only items the user hasn't acted on yet count toward the unread indicator —
	// disputes are waiting on a moderator, not on the author.
	let pendingCount = $derived(notifications.items.filter((i) => !i.disputed).length);
	// Combined unread = pending moderation items + unseen followers (Activity tab).
	let unreadCount = $derived(pendingCount + social.unseenCount);

	// One-time "new feature" indicators (server-backed). Gate on `loaded` so we
	// never flash a dot before knowing what the user has already seen.
	let showPinterestNew = $derived(features.loaded && !isFeatureSeen(FEATURE_PINTEREST_IMPORT));
	let showBlueskyImportNew = $derived(features.loaded && !isFeatureSeen(FEATURE_BLUESKY_IMPORT));
	let hasFeatureDot = $derived(features.loaded && hasUnseenAnnouncement());

	function openPinterestImport() {
		markFeatureSeen(FEATURE_PINTEREST_IMPORT);
		goto('/import/pinterest');
	}

	// Fetch the pending-attestation list once when the user is known. The store
	// caches across navigations; opening the dialog refreshes it again.
	onMount(() => {
		if (user) {
			void refreshNotifications();
			void refreshSocial();
			if (!features.loaded) void loadSeenFeatures();
			if (!modPrefsLoaded.value) void loadModerationPrefs();
		}
	});

	function handleCollectionCreated(collection: CollectionView) {
		addCollection(collection);
		toast.success(`Collection "${collection.name}" created`);
	}

	const native = isNative();

	async function handleLogout() {
		if (native) {
			try {
				await apiFetch('/oauth/logout');
			} catch {
				// best effort; even if the server call fails we still clear local state
			}
			await clearAuthToken();
			auth.user = null;
			auth.checked = true;
			goto('/');
		} else {
			window.location.href = `${PUBLIC_APPVIEW_URL}/oauth/logout`;
		}
	}

	function handleBrowserExtension() {
		const browser = detectBrowser();
		if (browser === 'firefox') {
			window.open(
				'https://addons.mozilla.org/en-US/firefox/addon/save-to-currents/',
				'_blank',
				'noopener'
			);
		} else if (browser === 'safari') {
			browserExtensionDialogOpen = true;
		} else {
			window.open(
				'https://chromewebstore.google.com/detail/save-to-currents/kdifjldjjhopgdhppjpknloichglmmdi',
				'_blank',
				'noopener'
			);
		}
	}

	$effect(() => {
		if (page.url.pathname.startsWith('/explore') || page.url.pathname === '/') query = '';
		else if (page.params.query) query = page.params.query;
		const t = page.params.type;
		if (t === 'collections' || t === 'users' || t === 'saves') searchType = t;
	});

	function onsubmit(e: Event) {
		e.preventDefault();
		const trimmed = query.trim();
		if (!trimmed) return;
		goto(
			resolve('/(with-navbar)/search/[type]/[query]', {
				type: searchType,
				query: encodeURIComponent(trimmed)
			})
		);
	}
</script>

{#snippet searchBar(autofocus: boolean, compact: boolean)}
	<InputGroup.Root
		class="{landing ? 'bg-accent/50 backdrop-blur-sm' : ''} {compact
			? 'h-9'
			: 'h-11'} w-full rounded-full"
	>
		<InputGroup.Addon align="inline-start">
			<InputGroup.Button
				size="icon-sm"
				aria-label="Search"
				disabled
				class="rounded-full text-muted-foreground"
			>
				<SearchIcon class="size-4" />
			</InputGroup.Button>
		</InputGroup.Addon>

		<InputGroup.Input
			type="search"
			placeholder="Search..."
			bind:value={query}
			{autofocus}
			autocorrect="off"
			autocapitalize="off"
			autocomplete="off"
			spellcheck={false}
			class={landing ? 'placeholder:text-white/70' : ''}
		/>

		<InputGroup.Addon align="inline-end">
			<Select.Root type="single" bind:value={searchType}>
				<Select.Trigger
					class="h-8 gap-1 rounded-full border-0 bg-transparent px-2.5 text-muted-foreground shadow-none hover:bg-accent focus-visible:ring-0"
				>
					{searchLabel}
				</Select.Trigger>
				<Select.Content align="end" class="rounded-2xl">
					{#each SEARCH_TYPES as t (t.value)}
						<Select.Item value={t.value} label={t.label}>{t.label}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</InputGroup.Addon>
	</InputGroup.Root>
{/snippet}

<Tooltip.Provider>
	<header
		class="{landing
			? 'fixed bg-transparent'
			: 'sticky app-muted-wash backdrop-blur-sm'} relative top-0 z-10 flex min-h-15 w-full items-center gap-3 px-2 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-3 md:px-4"
	>
		{#if !searchOpen}
			<div in:fade={{ duration: 250, easing: cubicOut }} class="flex shrink-0 items-center gap-2">
				<a href={resolve('/')} class="h-5 text-lg font-semibold text-foreground"><Logo /></a>
				<Tooltip.Root>
					<Tooltip.Trigger>
						<Badge variant="outline" class="cursor-default px-1.5 py-0 text-[10px] font-medium"
							>alpha</Badge
						>
					</Tooltip.Trigger>
					<Tooltip.Content>
						<p>Currents is early-stage software. Expect rough edges and breaking changes.</p>
					</Tooltip.Content>
				</Tooltip.Root>
			</div>
		{/if}

		<div
			class="absolute inset-y-0 left-1/2 hidden w-full -translate-x-1/2 items-center justify-center md:flex md:max-w-sm lg:max-w-md"
		>
			<form {onsubmit} class="w-full md:max-w-xs lg:max-w-sm">
				{@render searchBar(false, false)}
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
			<!-- Flow content (not absolute) so the header grows to fit the input and inherits the
			     header's safe-area top padding + bottom padding, instead of overflowing its box. -->
			<div
				transition:fade={{ duration: 250, easing: cubicOut }}
				class="flex flex-1 items-center gap-2 md:hidden"
			>
				<form {onsubmit} class="flex-1">
					{@render searchBar(true, true)}
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
					<DropdownMenu.Trigger class="relative shrink-0 rounded-full outline-none">
						<Avatar.Root size="default">
							{#if user.avatar}
								<Avatar.Image src={user.avatar} alt={user.displayName ?? user.handle} />
							{/if}
							<Avatar.Fallback>
								<UserIcon class="size-4" />
							</Avatar.Fallback>
						</Avatar.Root>
						{#if unreadCount > 0 || hasFeatureDot}
							<span
								class="absolute -top-0.5 -right-0.5 inline-flex h-2.5 w-2.5 rounded-full bg-red-500 ring-2 ring-background"
								aria-label={unreadCount > 0 ? `${unreadCount} unread` : 'New feature available'}
							></span>
						{/if}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-56">
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
							{#if showBlueskyImportNew}
								<Badge class="ml-auto bg-red-500/15 text-red-700 dark:text-red-300">New</Badge>
							{/if}
						</DropdownMenu.Item>
						<DropdownMenu.Item onclick={() => (notificationsOpen = true)}>
							<Bell class="size-4" />
							<span>Notifications</span>
							{#if unreadCount > 0}
								<Badge class="ml-auto bg-red-500/15 text-red-700 dark:text-red-300">
									{unreadCount}
								</Badge>
							{/if}
						</DropdownMenu.Item>
						<DropdownMenu.Item onclick={openPinterestImport}>
							<Download class="size-4" />
							Import from Pinterest
							{#if showPinterestNew}
								<Badge class="ml-auto bg-red-500/15 text-red-700 dark:text-red-300">New</Badge>
							{/if}
						</DropdownMenu.Item>
						{#if !native}
							<DropdownMenu.Item onclick={handleBrowserExtension}>
								<Puzzle class="size-4" />
								Browser extension
							</DropdownMenu.Item>
						{/if}
						<DropdownMenu.Item onclick={() => goto('/settings')}>
							<Settings class="size-4" />
							Settings
						</DropdownMenu.Item>
						<DropdownMenu.Item onclick={handleLogout}>
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
</Tooltip.Provider>

{#if user}
	<CollectionCreateDialog bind:open={createCollectionOpen} onCreated={handleCollectionCreated} />
	<BrowserExtensionDialog bind:open={browserExtensionDialogOpen} />
	<NotificationsDialog bind:open={notificationsOpen} />
{/if}
