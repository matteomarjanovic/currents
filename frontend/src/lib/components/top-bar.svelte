<script lang="ts">
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { apiFetch, logoutUrl } from '$lib/api';
	import { clearAuthToken } from '$lib/auth-storage';
	import { isNative, isAndroid, isIos, isMobileWeb, isStandalonePwa } from '$lib/platform';
	import { shouldOpenExternally, openExternal } from '$lib/external';
	import { pwaInstall, promptInstall } from '$lib/stores/pwa-install.svelte';
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
	import SearchLensIcon from '$lib/components/search-lens-icon.svelte';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
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
		FEATURE_BECOME_SUPPORTER,
		FEATURE_COLOR_SEARCH
	} from '$lib/stores/features.svelte';
	import { supporter, loadSupporterStatus } from '$lib/stores/supporter.svelte';
	import { navHistory } from '$lib/stores/navigation.svelte';
	import { openSettings } from '$lib/stores/settings.svelte';
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
	// Which cluster menu is open, if any. Opening one closes the rest: a touch tap
	// on a second trigger never reaches bits-ui's interact-outside handler, so
	// without this the first menu stays open behind the second one on phones
	// (mouse clicks close it fine, which is why this only shows up on touch).
	// Ids are per instance — the desktop and mobile clusters are both in the DOM.
	let openMenu = $state<string | null>(null);
	function toggleMenu(id: string, open: boolean) {
		if (open) openMenu = id;
		// Ignore the close that fires on the menu we just superseded.
		else if (openMenu === id) openMenu = null;
	}
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
	// Color search lives behind the palette toggle inside the search command, so
	// the dot on the search button is what leads there; the toggle carries its
	// own dot and clears the flag once the panel opens.
	let showColorSearchNew = $derived(
		!!user && features.loaded && !isFeatureSeen(FEATURE_COLOR_SEARCH)
	);

	// The supporter item in the avatar menu — a "Become a supporter" CTA for
	// non-supporters, or a "You're a supporter" badge for existing ones.
	// The one-time "new" indicator only makes sense for the CTA, not existing supporters.
	let showSupporterNew = $derived(
		features.loaded && !supporter.subscribed && !isFeatureSeen(FEATURE_BECOME_SUPPORTER)
	);

	// Each menu carries its own dot: the avatar aggregates notifications + items
	// inside the profile menu, the burger aggregates its own "new" items.
	let avatarDot = $derived(unreadCount > 0 || showBlueskyImportNew || showSupporterNew);
	let burgerDot = $derived(showPinterestNew);

	// What's currently being searched, surfaced next to the search button so the
	// query stays visible after the command dialog closes. Color searches carry the
	// hex (and paint the lens); hybrid searches carry both text and hex.
	let activeSearch = $derived.by(() => {
		if (page.route.id !== '/(with-navbar)/search/[type]/[query]') return null;
		const param = page.params.query ?? '';
		if (page.params.type === 'color') {
			return { text: page.url.searchParams.get('q') ?? '', color: '#' + param };
		}
		return param ? { text: param, color: '' } : null;
	});

	// Every page except the explore home gets a floating back button next to the
	// logo (save-detail has its own and doesn't render the top bar).
	let showBack = $derived(
		!landing && page.url.pathname !== '/' && !page.url.pathname.startsWith('/explore')
	);
	function goBack() {
		// Once the app has navigated internally at least once, the previous history entry is
		// guaranteed to be a Currents page (SvelteKit only pushes state for its own navigations).
		if (navHistory.hasInternalHistory) {
			history.back();
			return;
		}
		// Otherwise this is the tab's entry page (e.g. a shared link from search or social) — only
		// trust the browser back button if it would actually land back on Currents.
		const cameFromCurrents =
			!!document.referrer && new URL(document.referrer).hostname === location.hostname;
		if (cameFromCurrents && history.length > 1) history.back();
		else goto(resolve('/(with-navbar)/explore'));
	}

	function openPinterestImport() {
		markFeatureSeen(FEATURE_PINTEREST_IMPORT);
		goto(resolve('/(with-navbar)/import/pinterest'));
	}

	// Public pages render before the browser auth refresh finishes, so `user` can become known
	// after this component mounts. Load the account state once that happens.
	let loadedForDid: string | null = null;
	$effect(() => {
		if (!user || loadedForDid === user.did) return;
		loadedForDid = user.did;
		void refreshNotifications();
		void refreshSocial();
		if (!features.loaded) void loadSeenFeatures();
		if (!supporter.loaded) void loadSupporterStatus();
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
			// Full reload of the local bundle wipes every store (and the layout's own `user`
			// state), mirroring the web full-reload logout — a plain goto leaves stale state
			// that keeps the app looking logged in.
			window.location.href = '/';
		} else {
			window.location.href = logoutUrl();
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

	// The blog is prerendered outside the SPA — open it in the system browser from the native
	// app / PWA, otherwise navigate in-app.
	function openBlog() {
		if (shouldOpenExternally()) openExternal('/blog');
		else goto(resolve('/blog'));
	}

	// /support-us (external Polar checkout) is hidden from the native apps for App Store
	// compliance — it redirects to / on native. Google Play permits linking out, so on Android
	// open the page in the system browser where the checkout works; iOS can't sell in-app at
	// all, so it opens the settings Subscription section (perks + manage-subscription) instead.
	function openSupportUs() {
		if (!supporter.subscribed) markFeatureSeen(FEATURE_BECOME_SUPPORTER);
		if (isAndroid()) openExternal('/support-us');
		else if (isIos()) openSettings('subscription');
		else goto(resolve('/support-us'));
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

{#snippet searchQuery()}
	{#if activeSearch?.text}
		<span class="min-w-0 truncate">{activeSearch.text}</span>
	{/if}
	{#if activeSearch?.color}
		<span class="shrink-0 font-mono text-xs text-muted-foreground uppercase">
			{activeSearch.color}
		</span>
	{/if}
{/snippet}

<!-- The search trigger. With `labelled`, an active query sits to the left of the
     lens — desktop only, since the mobile bar has no room; there the query rides
     above the bottom cluster instead. -->
{#snippet searchButton(variant: 'ghost' | 'glass', extraClass: string, labelled: boolean)}
	{@const withQuery = labelled && !!activeSearch}
	<Button
		{variant}
		size="icon"
		class="relative rounded-full {withQuery
			? 'md:w-auto md:max-w-72 md:gap-1.5 md:px-3'
			: ''} {extraClass}"
		type="button"
		aria-label="Search"
		onclick={() => (searchCommandOpen = true)}
	>
		{#if withQuery}
			<!-- Baseline, not center: the hex is a size smaller than the query text, so
			     centering the two boxes leaves their glyphs sitting a pixel apart. -->
			<span class="hidden min-w-0 items-baseline gap-1.5 md:flex">{@render searchQuery()}</span>
		{/if}
		<SearchLensIcon class="size-4" color={activeSearch?.color} />
		{#if showColorSearchNew}
			<span
				class="absolute top-0 right-0 inline-flex h-2.5 w-2.5 rounded-full bg-red-500 ring-2 ring-background"
				aria-label="New feature available"
			></span>
		{/if}
	</Button>
{/snippet}

{#snippet loginButton(size: 'default' | 'lg')}
	<a href={resolve('/login')} class="pointer-events-auto shrink-0">
		<Button variant="default" {size} class="rounded-full px-5">Log in</Button>
	</a>
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

{#snippet plusMenu(
	id: string,
	side: 'top' | 'bottom',
	align: 'center' | 'end',
	anchor?: HTMLElement
)}
	<DropdownMenu.Root bind:open={() => openMenu === id, (v) => toggleMenu(id, v)}>
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

{#snippet avatarMenu(
	id: string,
	side: 'top' | 'bottom',
	align: 'center' | 'end',
	anchor?: HTMLElement
)}
	{#if user}
		<DropdownMenu.Root bind:open={() => openMenu === id, (v) => toggleMenu(id, v)}>
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
				<DropdownMenu.Item onclick={openSupportUs}>
					<Heart class="size-4 fill-pink-500 stroke-pink-500" />
					{supporter.subscribed ? "You're a supporter" : 'Become a supporter'}
					{#if showSupporterNew}
						<Badge class="ml-auto bg-red-500/15 text-red-700 dark:text-red-300">New</Badge>
					{/if}
				</DropdownMenu.Item>
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
	<DropdownMenu.Item onclick={openBlog}>
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
		: 'sticky'} pointer-events-none top-0 z-10 flex min-h-[calc(3.75rem+env(safe-area-inset-top))] w-full items-center gap-2 bg-transparent px-2 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-3 md:px-4"
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
				{#if user}
					<ModeSwitcher
						mode="explore"
						bind:open={() => openMenu === 'mode-desktop', (v) => toggleMenu('mode-desktop', v)}
					/>
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
				{@render searchButton('ghost', '', true)}
				{@render plusMenu('plus-desktop', 'bottom', 'end')}
				<DropdownMenu.Root
					bind:open={() => openMenu === 'burger-desktop', (v) => toggleMenu('burger-desktop', v)}
				>
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
								{@render burgerIcon(openMenu === 'burger-desktop')}
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
				{@render avatarMenu('avatar-desktop', 'bottom', 'end')}
			</div>
		{:else if landing}
			<!-- The hero already has a search bar on desktop; on mobile the icon expands
			     it inline rather than opening the command dialog. -->
			<Button
				variant="ghost"
				size="icon"
				class="pointer-events-auto shrink-0 rounded-full md:hidden"
				type="button"
				aria-label="Search"
				onclick={() => (searchOpen = true)}
			>
				<SearchIcon class="size-4" />
			</Button>
			{@render loginButton('lg')}
		{:else}
			<!-- Desktop top-right cluster for logged-out viewers. On mobile these live in
			     the bottom cluster instead, mirroring the logged-in bar. -->
			<div class="hidden shrink-0 items-center gap-2 md:flex">
				{@render searchButton('glass', 'pointer-events-auto shrink-0', true)}
				<ThemeToggle
					class="pointer-events-auto h-9 shrink-0 rounded-full bg-primary-foreground/80 text-foreground shadow-sm backdrop-blur-sm hover:bg-primary-foreground aria-expanded:bg-primary-foreground"
				/>
				{@render loginButton('lg')}
			</div>
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

<!-- Mobile bottom-center cluster. Logged in: profile, menu, add, mode switch, search;
     logged out: log in, theme, search. Scale the compact controls up slightly so
     the bar and its icons are easier to hit without changing its composition. -->
{#if !landing}
	{#if activeSearch}
		<!-- The bottom cluster has no room beside the lens, so the active query rides
		     just above it (44px cluster + 0.5rem gap over the cluster's own offset). -->
		<button
			type="button"
			onclick={() => (searchCommandOpen = true)}
			class="fixed left-1/2 z-10 flex max-w-[calc(100vw-2rem)] -translate-x-1/2 items-baseline gap-1.5 rounded-full border border-transparent bg-primary-foreground/80 bg-clip-padding px-3 py-1.5 text-sm text-foreground shadow-sm backdrop-blur-sm md:hidden"
			style="bottom: calc(env(safe-area-inset-bottom) + 4.625rem)"
		>
			{@render searchQuery()}
		</button>
	{/if}
	<div
		bind:this={bottomBarEl}
		class="{glassGroup} fixed left-1/2 z-10 flex -translate-x-1/2 scale-[1.08] md:hidden"
		style="bottom: calc(env(safe-area-inset-bottom) + 1.375rem)"
	>
		{#if !user}
			{@render loginButton('default')}
			<!-- Bare like the logged-in cluster's ghost buttons: the trigger's own
			     bg-input/50 would read as a pressed state inside the glass pill. The
			     asymmetric padding is optical, not arithmetic: the chevron already
			     carries trailing space of its own, while the solid Log in pill needs
			     room to breathe — that lands both gaps at ~18px of visible space. -->
			<ThemeToggle
				class="h-9 gap-1 rounded-full bg-transparent pr-1 pl-4 text-foreground hover:bg-muted aria-expanded:bg-muted dark:hover:bg-muted/50"
			/>
		{:else}
			{@render avatarMenu('avatar-mobile', 'top', 'center', bottomBarEl)}
			<DropdownMenu.Root
				bind:open={() => openMenu === 'burger-mobile', (v) => toggleMenu('burger-mobile', v)}
			>
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
							{@render burgerIcon(openMenu === 'burger-mobile')}
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
			{@render plusMenu('plus-mobile', 'top', 'center', bottomBarEl)}
			{#if user}
				<ModeSwitcher
					mode="explore"
					variant="icon"
					side="top"
					anchor={bottomBarEl}
					bind:open={() => openMenu === 'mode-mobile', (v) => toggleMenu('mode-mobile', v)}
				/>
			{/if}
		{/if}
		{@render searchButton('ghost', '', false)}
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
