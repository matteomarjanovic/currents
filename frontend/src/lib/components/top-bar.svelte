<script lang="ts">
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { apiFetch } from '$lib/api';
	import { clearAuthToken } from '$lib/auth-storage';
	import { isNative, isMobileWeb, isStandalonePwa } from '$lib/platform';
	import { pwaInstall, promptInstall } from '$lib/stores/pwa-install.svelte';
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
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import X from '@lucide/svelte/icons/x';
	import Plus from '@lucide/svelte/icons/plus';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import Puzzle from '@lucide/svelte/icons/puzzle';
	import Smartphone from '@lucide/svelte/icons/smartphone';
	import Newspaper from '@lucide/svelte/icons/newspaper';
	import Settings from '@lucide/svelte/icons/settings';
	import Bell from '@lucide/svelte/icons/bell';
	import Heart from '@lucide/svelte/icons/heart';
	import Logo from '$lib/assets/logo.svelte';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import CollectionCreateDialog from '$lib/components/collection-create-dialog.svelte';
	import BrowserExtensionDialog from '$lib/components/browser-extension-dialog.svelte';
	import InstallAppDialog from '$lib/components/install-app-dialog.svelte';
	import NotificationsDialog from '$lib/components/notifications-dialog.svelte';
	import ModeSwitcher from '$lib/components/mode-switcher.svelte';
	import SearchCommand from '$lib/components/search-command.svelte';
	import { addCollection } from '$lib/stores/collections.svelte';
	import { notifications, refreshNotifications } from '$lib/stores/notifications.svelte';
	import { social, refreshSocial } from '$lib/stores/social.svelte';
	import {
		features,
		loadSeenFeatures,
		markFeatureSeen,
		isFeatureSeen,
		FEATURE_PINTEREST_IMPORT,
		FEATURE_BLUESKY_IMPORT,
		FEATURE_BECOME_SUPPORTER
	} from '$lib/stores/features.svelte';
	import { loadModerationPrefs, modPrefsLoaded } from '$lib/stores/moderation-prefs.svelte';
	import { role, loadRole, previewGated, canSeePreviewFeatures } from '$lib/stores/role.svelte';
	import { supporter, loadSupporterStatus } from '$lib/stores/supporter.svelte';
	import { openSettings } from '$lib/stores/settings.svelte';
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
	let searchCommandOpen = $state(false);
	let createCollectionOpen = $state(false);
	let browserExtensionDialogOpen = $state(false);
	let installAppDialogOpen = $state(false);
	let notificationsOpen = $state(false);
	let burgerOpenDesktop = $state(false);
	let burgerOpenMobile = $state(false);
	// The mobile bottom bar; its menus anchor to it (centered) instead of to their buttons.
	let bottomBarEl = $state<HTMLElement | undefined>();

	// Only items the user hasn't acted on yet count toward the unread indicator —
	// disputes are waiting on a moderator, not on the author.
	let pendingCount = $derived(notifications.items.filter((i) => !i.disputed).length);
	// Combined unread = pending moderation items + unseen followers (Activity tab).
	let unreadCount = $derived(pendingCount + social.unseenCount);

	// One-time "new feature" indicators (server-backed). Gate on `loaded` so we
	// never flash a dot before knowing what the user has already seen.
	let showPinterestNew = $derived(features.loaded && !isFeatureSeen(FEATURE_PINTEREST_IMPORT));
	let showBlueskyImportNew = $derived(features.loaded && !isFeatureSeen(FEATURE_BLUESKY_IMPORT));

	// Organize mode is preview-gated to moderators until public launch; the mode
	// switcher only shows when the viewer can actually enter it.
	let canSeeOrganize = $derived(!!user && (!previewGated || role.value != null));

	// The supporter item in the avatar menu — a "Become a supporter" CTA for
	// non-supporters, or a "You're a supporter" badge for existing ones. Visible
	// to everyone (while preview-gated) who can reach the subscription settings.
	let showSupporterItem = $derived(canSeePreviewFeatures());
	// The one-time "new" indicator only makes sense for the CTA, not existing supporters.
	let showSupporterNew = $derived(
		features.loaded &&
			showSupporterItem &&
			!supporter.subscribed &&
			!isFeatureSeen(FEATURE_BECOME_SUPPORTER)
	);

	// Each menu carries its own dot: the avatar aggregates notifications + items
	// inside the profile menu, the burger aggregates its own "new" items.
	let avatarDot = $derived(unreadCount > 0 || showBlueskyImportNew || showSupporterNew);
	let burgerDot = $derived(showPinterestNew);

	// Every page except the explore home gets a floating back button next to the
	// logo (save-detail has its own and doesn't render the top bar).
	let showBack = $derived(
		!landing && page.url.pathname !== '/' && !page.url.pathname.startsWith('/explore')
	);
	function goBack() {
		if (typeof history !== 'undefined' && history.length > 1) history.back();
		else goto(resolve('/'));
	}

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
			if (previewGated && !role.loaded) void loadRole();
			if (!supporter.loaded) void loadSupporterStatus();
		}
	});

	function handleCollectionCreated(collection: CollectionView) {
		addCollection(collection);
		toast.success(`Collection "${collection.name}" created`);
	}

	const native = isNative();
	// On mobile web, offer "Install app" (PWA) instead of the browser-extension item, which is
	// desktop-only. Hide both inside the native shell and once running as an installed PWA.
	const mobileWeb = !native && isMobileWeb();
	const standalone = isStandalonePwa();
	const showExtension = !native && !mobileWeb;
	let showInstall = $derived(mobileWeb && !standalone && !pwaInstall.installed);

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

	async function handleInstallApp() {
		// Chromium gives us a deferred prompt to fire directly; otherwise (iOS, or no prompt
		// captured yet) fall back to platform instructions.
		if (pwaInstall.canPrompt) {
			await promptInstall();
		} else {
			installAppDialogOpen = true;
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

	// Shared shell for the floating button clusters (add display per instance).
	const glassGroup =
		'pointer-events-auto shrink-0 items-center gap-0.5 rounded-full border border-transparent bg-primary-foreground/80 bg-clip-padding p-1 shadow-sm backdrop-blur-sm';
</script>

{#snippet searchBar(autofocus: boolean, compact: boolean)}
	<InputGroup.Root
		class="{landing
			? 'bg-accent/50 backdrop-blur-sm'
			: 'bg-primary-foreground/80 shadow-sm backdrop-blur-sm'} {compact
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

{#snippet burgerIcon(open: boolean)}
	<span class="relative block size-4">
		<span
			class="absolute top-[4px] left-0 h-0.5 w-4 rounded-full bg-current transition-transform duration-200 {open
				? 'translate-y-[3px] rotate-45'
				: ''}"
		></span>
		<span
			class="absolute top-[10px] left-0 h-0.5 w-4 rounded-full bg-current transition-transform duration-200 {open
				? '-translate-y-[3px] -rotate-45'
				: ''}"
		></span>
	</span>
{/snippet}

{#snippet plusMenu(side: 'top' | 'bottom', align: 'center' | 'end', anchor?: HTMLElement)}
	<DropdownMenu.Root>
		<DropdownMenu.Trigger class="shrink-0 outline-none">
			{#snippet child({ props })}
				<Button
					{...props}
					variant="ghost"
					size="icon"
					class="rounded-full"
					type="button"
					aria-label="Add"
				>
					<Plus class="size-5" />
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content {side} {align} customAnchor={anchor ?? null} class="w-48">
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
{/snippet}

{#snippet avatarMenu(side: 'top' | 'bottom', align: 'center' | 'end', anchor?: HTMLElement)}
	{#if user}
		<DropdownMenu.Root>
			<DropdownMenu.Trigger
				aria-label="Profile menu"
				class="relative flex size-9 shrink-0 items-center justify-center rounded-full outline-none"
			>
				<Avatar.Root size="default">
					{#if user.avatar}
						<Avatar.Image src={user.avatar} alt={user.displayName ?? user.handle} />
					{/if}
					<Avatar.Fallback>
						<UserIcon class="size-4" />
					</Avatar.Fallback>
				</Avatar.Root>
				{#if avatarDot}
					<span
						class="absolute top-0 right-0 inline-flex h-2.5 w-2.5 rounded-full bg-red-500 ring-2 ring-background"
						aria-label={unreadCount > 0 ? `${unreadCount} unread` : 'New feature available'}
					></span>
				{/if}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content {side} {align} customAnchor={anchor ?? null} class="w-56">
				<DropdownMenu.Label>
					{#if user.displayName}
						<div class="text-base text-primary">{user.displayName}</div>
					{/if}
					<div class="font-normal text-muted-foreground">@{user.handle}</div>
				</DropdownMenu.Label>
				<DropdownMenu.Separator />
				<DropdownMenu.Item
					onclick={() => goto(resolve('/(with-navbar)/profile/[handle]', { handle: user.handle }))}
				>
					<UserIcon class="size-4" />
					Go to profile
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
				{#if showSupporterItem}
					<DropdownMenu.Item
						onclick={() => {
							if (!supporter.subscribed) markFeatureSeen(FEATURE_BECOME_SUPPORTER);
							goto(resolve('/support-us'));
						}}
					>
						<Heart class="size-4 fill-pink-500 stroke-pink-500" />
						{supporter.subscribed ? "You're a supporter" : 'Become a supporter'}
						{#if showSupporterNew}
							<Badge class="ml-auto bg-red-500/15 text-red-700 dark:text-red-300">New</Badge>
						{/if}
					</DropdownMenu.Item>
				{/if}
				<DropdownMenu.Separator />
				<DropdownMenu.Item onclick={handleLogout}>
					<LogOut class="size-4" />
					Log out
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	{/if}
{/snippet}

{#snippet burgerItems()}
	<DropdownMenu.Item onclick={openPinterestImport}>
		<Download class="size-4" />
		Import from Pinterest
		{#if showPinterestNew}
			<Badge class="ml-auto bg-red-500/15 text-red-700 dark:text-red-300">New</Badge>
		{/if}
	</DropdownMenu.Item>
	{#if showInstall}
		<DropdownMenu.Item onclick={handleInstallApp}>
			<Smartphone class="size-4" />
			Install app
		</DropdownMenu.Item>
	{:else if showExtension}
		<DropdownMenu.Item onclick={handleBrowserExtension}>
			<Puzzle class="size-4" />
			Get browser extension
		</DropdownMenu.Item>
	{/if}
	<DropdownMenu.Item onclick={() => goto(resolve('/blog'))}>
		<Newspaper class="size-4" />
		Blog
	</DropdownMenu.Item>
	<DropdownMenu.Item onclick={() => openSettings()}>
		<Settings class="size-4" />
		Settings
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
{/snippet}

<header
	class="{landing
		? 'fixed'
		: 'sticky'} pointer-events-none top-0 z-10 flex min-h-15 w-full items-center gap-2 bg-transparent px-2 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-3 md:px-4"
>
	{#if !searchOpen}
		{#if landing}
			<a
				in:fade={{ duration: 250, easing: cubicOut }}
				href={resolve('/')}
				class="pointer-events-auto h-5 shrink-0 text-lg font-semibold text-foreground"><Logo /></a
			>
		{:else}
			<div
				in:fade={{ duration: 250, easing: cubicOut }}
				class="pointer-events-auto hidden shrink-0 items-center gap-2 md:flex"
			>
				{#if showBack}
					<Button
						variant="glass"
						size="icon-lg"
						class="size-11 rounded-full"
						onclick={goBack}
						aria-label="Go back"
					>
						<ArrowLeft class="size-5" />
					</Button>
				{/if}
				<a
					href={resolve('/')}
					aria-label="Go to home"
					class="flex h-11 shrink-0 items-center rounded-full border border-transparent bg-primary-foreground/80 bg-clip-padding px-4 text-foreground shadow-sm backdrop-blur-sm"
				>
					<span class="block h-5"><Logo /></span>
				</a>
				{#if canSeeOrganize}
					<ModeSwitcher mode="explore" />
				{/if}
			</div>
			<!-- Mobile: the logo floats centered on its own (same box and position as the
			     save-detail home button, so it doesn't jump between views); the buttons
			     live in the bottom cluster instead. -->
			<a
				in:fade={{ duration: 250, easing: cubicOut }}
				href={resolve('/')}
				aria-label="Go to home"
				class="pointer-events-auto fixed left-1/2 flex -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-transparent bg-primary-foreground/80 bg-clip-padding px-4 py-2.5 text-foreground shadow-sm backdrop-blur-sm md:hidden"
				style="top: calc(env(safe-area-inset-top) + 2rem)"
			>
				<span class="block h-5"><Logo /></span>
			</a>
			{#if showBack}
				<!-- The positioning translate lives on a wrapper: on the button itself the
				     pressed-state translate-y-px would override it and jump the button down. -->
				<div
					class="pointer-events-auto fixed left-2 -translate-y-1/2 md:hidden"
					style="top: calc(env(safe-area-inset-top) + 2rem)"
				>
					<Button
						variant="glass"
						size="icon-lg"
						class="size-11 rounded-full"
						onclick={goBack}
						aria-label="Go back"
					>
						<ArrowLeft class="size-5" />
					</Button>
				</div>
			{/if}
		{/if}
	{/if}

	{#if landing}
		<!-- The landing hero keeps the prominent search bar; everywhere else search
		     lives behind an icon button that opens the search command. -->
		<div
			class="pointer-events-auto absolute inset-y-0 left-1/2 hidden w-full -translate-x-1/2 items-center justify-center md:flex md:max-w-sm lg:max-w-md"
		>
			<form {onsubmit} class="w-full md:max-w-xs lg:max-w-sm">
				{@render searchBar(false, false)}
			</form>
		</div>
	{/if}

	{#if !searchOpen}
		<div class="flex-1"></div>

		{#if user}
			<!-- Desktop top-right cluster: search, add, burger, profile. On mobile these
			     live in the bottom cluster instead. -->
			<div in:fade={{ duration: 250, easing: cubicOut }} class="{glassGroup} hidden md:flex">
				<Button
					variant="ghost"
					size="icon"
					class="rounded-full"
					type="button"
					aria-label="Search"
					onclick={() => (searchCommandOpen = true)}
				>
					<SearchIcon class="size-4" />
				</Button>
				{@render plusMenu('bottom', 'end')}
				<DropdownMenu.Root bind:open={burgerOpenDesktop}>
					<DropdownMenu.Trigger class="shrink-0 outline-none">
						{#snippet child({ props })}
							<Button
								{...props}
								variant="ghost"
								size="icon"
								class="relative rounded-full"
								type="button"
								aria-label="Menu"
							>
								{@render burgerIcon(burgerOpenDesktop)}
								{#if burgerDot}
									<span
										class="absolute top-0 right-0 inline-flex h-2.5 w-2.5 rounded-full bg-red-500 ring-2 ring-background"
										aria-label="New feature available"
									></span>
								{/if}
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-56">
						{@render burgerItems()}
					</DropdownMenu.Content>
				</DropdownMenu.Root>
				{@render avatarMenu('bottom', 'end')}
			</div>
		{:else}
			<Button
				variant={landing ? 'ghost' : 'glass'}
				size="icon"
				class="pointer-events-auto shrink-0 rounded-full {landing ? 'md:hidden' : ''}"
				type="button"
				aria-label="Search"
				onclick={() => (landing ? (searchOpen = true) : (searchCommandOpen = true))}
			>
				<SearchIcon class="size-4" />
			</Button>
			<a href={resolve('/login')} class="pointer-events-auto">
				<Button variant="default" size="lg" class="shrink-0 rounded-full px-5">Log in</Button>
			</a>
		{/if}
	{/if}

	{#if searchOpen}
		<!-- Flow content (not absolute) so the header grows to fit the input and inherits the
		     header's safe-area top padding + bottom padding, instead of overflowing its box. -->
		<div
			transition:fade={{ duration: 250, easing: cubicOut }}
			class="pointer-events-auto flex flex-1 items-center gap-2 md:hidden"
		>
			<form {onsubmit} class="flex-1">
				{@render searchBar(true, true)}
			</form>
			<Button
				variant="glass"
				size="icon"
				class="shrink-0 rounded-full"
				onclick={() => (searchOpen = false)}
			>
				<X class="size-4" />
			</Button>
		</div>
	{/if}
</header>

<!-- Mobile bottom-center cluster: profile, menu, add, mode switch, search. The extra
     0.125rem centers the 44px-tall cluster on the 48px explore flow-field button. -->
{#if user && !landing}
	<div
		bind:this={bottomBarEl}
		class="{glassGroup} fixed left-1/2 z-10 flex -translate-x-1/2 md:hidden"
		style="bottom: calc(env(safe-area-inset-bottom) + 1.375rem)"
	>
		{@render avatarMenu('top', 'center', bottomBarEl)}
		<DropdownMenu.Root bind:open={burgerOpenMobile}>
			<DropdownMenu.Trigger class="shrink-0 outline-none">
				{#snippet child({ props })}
					<Button
						{...props}
						variant="ghost"
						size="icon"
						class="relative rounded-full"
						type="button"
						aria-label="Menu"
					>
						{@render burgerIcon(burgerOpenMobile)}
						{#if burgerDot}
							<span
								class="absolute top-0 right-0 inline-flex h-2.5 w-2.5 rounded-full bg-red-500 ring-2 ring-background"
								aria-label="New feature available"
							></span>
						{/if}
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content
				side="top"
				align="center"
				customAnchor={bottomBarEl ?? null}
				class="w-56"
			>
				{@render burgerItems()}
			</DropdownMenu.Content>
		</DropdownMenu.Root>
		{@render plusMenu('top', 'center', bottomBarEl)}
		{#if canSeeOrganize}
			<ModeSwitcher mode="explore" variant="icon" side="top" anchor={bottomBarEl} />
		{/if}
		<Button
			variant="ghost"
			size="icon"
			class="rounded-full"
			type="button"
			aria-label="Search"
			onclick={() => (searchCommandOpen = true)}
		>
			<SearchIcon class="size-4" />
		</Button>
	</div>
{/if}

<!-- ⌘K / Ctrl+K opens the search command from anywhere. -->
<svelte:window
	onkeydown={(e) => {
		if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
			e.preventDefault();
			searchCommandOpen = true;
		}
	}}
/>

<SearchCommand bind:open={searchCommandOpen} />

{#if user}
	<CollectionCreateDialog bind:open={createCollectionOpen} onCreated={handleCollectionCreated} />
	<BrowserExtensionDialog bind:open={browserExtensionDialogOpen} />
	<InstallAppDialog bind:open={installAppDialogOpen} />
	<NotificationsDialog bind:open={notificationsOpen} />
{/if}
