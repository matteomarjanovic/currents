<script lang="ts">
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { devicePixelRatio, scrollY } from 'svelte/reactivity/window';
	import { apiFetch, logoutUrl } from '$lib/api';
	import { clearAuthToken } from '$lib/auth-storage';
	import { isNative, isAndroid, isIos, isMobileWeb, isStandalonePwa } from '$lib/platform';
	import { shouldOpenExternally, openExternal } from '$lib/external';
	import { pwaInstall, promptInstall } from '$lib/stores/pwa-install.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Avatar from '$lib/components/ui/avatar';
	import { setMode, resetMode, userPrefersMode } from 'mode-watcher';
	import { fade, fly } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import LogOut from '@lucide/svelte/icons/log-out';
	import UserIcon from '@lucide/svelte/icons/user';
	import Download from '@lucide/svelte/icons/download';
	import Sun from '@lucide/svelte/icons/sun';
	import Moon from '@lucide/svelte/icons/moon';
	import Monitor from '@lucide/svelte/icons/monitor';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import Plus from '@lucide/svelte/icons/plus';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import Puzzle from '@lucide/svelte/icons/puzzle';
	import Smartphone from '@lucide/svelte/icons/smartphone';
	import Newspaper from '@lucide/svelte/icons/newspaper';
	import MessageSquare from '@lucide/svelte/icons/message-square';
	import Settings from '@lucide/svelte/icons/settings';
	import Bell from '@lucide/svelte/icons/bell';
	import Heart from '@lucide/svelte/icons/heart';
	import Logo from '$lib/assets/logo.svelte';
	import LogoIcon from '$lib/assets/logo-icon.svelte';
	import SearchLensIcon from '$lib/components/search-lens-icon.svelte';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import CollectionCreateDialog from '$lib/components/collection-create-dialog.svelte';
	import BrowserExtensionDialog from '$lib/components/browser-extension-dialog.svelte';
	import InstallAppDialog from '$lib/components/install-app-dialog.svelte';
	import NotificationsDialog from '$lib/components/notifications-dialog.svelte';
	import ModeSwitcher from '$lib/components/mode-switcher.svelte';
	import PersonalizationButton from '$lib/components/personalization-button-v3.svelte';
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
	import { openSettings } from '$lib/stores/settings.svelte';
	import { detectBrowser } from '$lib/browser';
	import type { CollectionView } from '$lib/types';
	import { mobileBottomBarState } from '$lib/mode-tabs.svelte';

	let {
		user,
		landing = false,
		fadeContent = false,
		showLogo = false,
		actionsOnly = false,
		mobileOnly = false,
		scrollbarInset = 0
	}: {
		user: { did: string; handle: string; displayName?: string; avatar?: string } | null;
		landing?: boolean;
		fadeContent?: boolean;
		showLogo?: boolean;
		actionsOnly?: boolean;
		mobileOnly?: boolean;
		scrollbarInset?: number;
	} = $props();

	// Android and web need the raised mobile bar; iOS keeps extra room for its native layout.
	const raisedMobileBar = isAndroid() || !isNative();
	const logoPillId = $props.id();
	let routeMode = $derived<'organize' | 'explore'>(
		page.url.pathname.startsWith('/organize') ? 'organize' : 'explore'
	);

	let searchCommandOpen = $state(false);
	let createCollectionOpen = $state(false);
	let browserExtensionDialogOpen = $state(false);
	let installAppDialogOpen = $state(false);
	let notificationsOpen = $state(false);
	let moderationConsent = $derived(page.url.searchParams.get('bluesky-moderation') ?? undefined);

	$effect(() => {
		if (user && moderationConsent) notificationsOpen = true;
	});
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
	let showFlowField = $derived(page.route.id === '/(with-navbar)/explore/[level]');
	const guestExplore = $derived(
		!actionsOnly &&
			!isNative() &&
			!user &&
			(showFlowField || page.route.id === '/(with-navbar)/explore')
	);

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

	// Mobile keeps a floating back button on every page except the explore home.
	let showBack = $derived(
		!landing && page.url.pathname !== '/' && !page.url.pathname.startsWith('/explore')
	);
	function goBack() {
		// Back means browser Back, including returning to the external page that opened Currents.
		// Only use the Explore fallback when this tab truly has no previous history entry.
		if (history.length > 1) history.back();
		else goto(resolve('explore'));
	}

	function openPinterestImport() {
		markFeatureSeen(FEATURE_PINTEREST_IMPORT);
		goto(resolve('import/pinterest'));
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
		else goto(resolve('blog'));
	}

	async function openFeedback() {
		if (native) {
			const { Browser } = await import('@capacitor/browser');
			await Browser.open({ url: 'https://currents.is/feedback' });
		} else {
			window.open('https://currents.is/feedback', '_blank', 'noopener');
		}
	}

	// /support-us (external Polar checkout) is hidden from the native apps for App Store
	// compliance — it redirects to / on native. Google Play permits linking out, so on Android
	// open the page in the system browser where the checkout works; iOS can't sell in-app at
	// all, so it opens the settings Subscription section (perks + manage-subscription) instead.
	function openSupportUs() {
		if (!supporter.subscribed) markFeatureSeen(FEATURE_BECOME_SUPPORTER);
		if (isAndroid()) openExternal('/support-us');
		else if (isIos()) openSettings('subscription');
		else goto(resolve('support-us'));
	}

	// Shared shell for the floating button clusters (add display per instance).
	const glassGroup =
		'pointer-events-auto shrink-0 items-center gap-0.5 rounded-full border bg-primary-foreground/80 bg-clip-padding p-1 backdrop-blur-sm';
	const floatingTheme =
		'pointer-events-auto size-10 shrink-0 rounded-full bg-primary-foreground/80 text-foreground shadow-sm backdrop-blur-sm hover:bg-primary-foreground aria-expanded:bg-primary-foreground';
</script>

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
	<Button href={resolve('login')} {size} class="pointer-events-auto shrink-0 rounded-full px-5">
		Log in
	</Button>
{/snippet}

{#snippet guestActions(showTheme: boolean)}
	<div class="pointer-events-auto flex h-12 shrink-0 items-center gap-2 sm:h-13">
		{#if showTheme}<ThemeToggle class={floatingTheme} />{/if}
		{@render searchButton('glass', 'size-10', false)}
		{@render loginButton('lg')}
	</div>
{/snippet}

{#snippet burgerIcon(open: boolean, mobile = false)}
	{@const ratio = devicePixelRatio.current ?? 1}
	<!-- Target 6 CSS pixels, snapping the separation to whole physical pixels. -->
	{@const offset = ((Math.round(6 * ratio) / ratio / 2) * 24) / (mobile ? 22 : 20)}
	<svg
		class="size-5"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		stroke-width="2"
		stroke-linecap="round"
		aria-hidden="true"
	>
		<line
			x1="5"
			y1={12 - offset}
			x2="19"
			y2={12 - offset}
			style:translate="0 {open ? offset : 0}px"
			class="origin-center transition-transform duration-200 [transform-box:fill-box] {open
				? 'rotate-45'
				: ''}"
		/>
		<line
			x1="5"
			y1={12 + offset}
			x2="19"
			y2={12 + offset}
			style:translate="0 {open ? -offset : 0}px"
			class="origin-center transition-transform duration-200 [transform-box:fill-box] {open
				? '-rotate-45'
				: ''}"
		/>
	</svg>
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
					<Plus
						class="size-5 transition-transform duration-200 motion-reduce:transition-none {openMenu ===
						id
							? 'rotate-45'
							: ''}"
					/>
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content {side} {align} customAnchor={anchor ?? null} class="w-48">
			<DropdownMenu.Item onclick={() => (createCollectionOpen = true)}>
				<FolderPlus class="size-4" />
				Create collection
			</DropdownMenu.Item>
			<DropdownMenu.Item onclick={() => goto(resolve('upload'))}>
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
	<DropdownMenu.Item onclick={openFeedback}>
		<MessageSquare class="size-4" />
		Leave feedback
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

{#snippet desktopActions()}
	<div
		data-desktop-actions
		in:fade={{ duration: 250, easing: cubicOut }}
		class="{glassGroup} hidden border-border shadow-md shadow-black/15 md:flex"
	>
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
{/snippet}

{#if actionsOnly && !mobileOnly}
	<header
		class="pointer-events-none fixed inset-x-0 top-0 z-50 flex min-h-[calc(3.75rem+env(safe-area-inset-top))] items-center px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-3"
		style:right="{scrollbarInset}px"
	>
		<div class="flex-1"></div>
		{#if user}{@render desktopActions()}{/if}
	</header>
{:else if landing}
	<header
		class="explore-top-bar landing-top-bar pointer-events-none fixed inset-x-0 top-0 z-20 flex items-center justify-between gap-2 px-3 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-3 md:px-6"
	>
		<a
			href={resolve('')}
			aria-label="Currents home"
			class="pointer-events-auto relative inline-flex h-12 shrink-0 items-center rounded-full px-3.5 text-foreground focus-visible:outline-2 sm:h-13 sm:px-4"
		>
			<span
				class="pointer-events-none absolute inset-0 bg-primary-foreground/80 backdrop-blur-sm"
				style:clip-path="url(#logo-pill-clip-{logoPillId})"
				aria-hidden="true"
			></span>
			<svg
				class="pointer-events-none absolute inset-0 size-full"
				viewBox="0 0 1 1"
				preserveAspectRatio="none"
				aria-hidden="true"
			>
				<defs>
					<!-- Capsule ends with a shallow bow along each long edge. -->
					<path
						id="logo-pill-{logoPillId}"
						d="M.15 .04 Q.5 .021 .85 .04 C.9375 .04475 .996 .24 .996 .5 C.996 .76 .9375 .95525 .85 .96 Q.5 .979 .15 .96 C.0625 .95525 .004 .76 .004 .5 C.004 .24 .0625 .04475 .15 .04 Z"
						vector-effect="non-scaling-stroke"
					/>
					<clipPath id="logo-pill-clip-{logoPillId}" clipPathUnits="objectBoundingBox">
						<use href="#logo-pill-{logoPillId}" />
					</clipPath>
				</defs>
				<use href="#logo-pill-{logoPillId}" fill="none" stroke="var(--border)" stroke-width="1" />
			</svg>
			<span class="relative block h-[1.125rem] sm:h-[1.375rem]"><Logo /></span>
		</a>
		{#if user}
			{@render desktopActions()}
		{:else}
			{@render guestActions(false)}
		{/if}
	</header>
{:else if !actionsOnly}
	<header
		data-scrolled={fadeContent && (scrollY.current ?? 0) > 10 ? '' : undefined}
		data-guest-explore={guestExplore ? '' : undefined}
		class="{fadeContent
			? 'explore-top-bar'
			: ''} pointer-events-none sticky top-0 z-10 flex min-h-[calc(3.75rem+env(safe-area-inset-top))] w-full items-center gap-2 bg-transparent pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-3 {guestExplore
			? 'px-3 md:px-6'
			: 'px-2 md:px-4'}"
	>
		{#if showLogo}
			<a
				in:fade={{ duration: 250, easing: cubicOut }}
				href={resolve('')}
				aria-label="Go to home"
				class="pointer-events-auto hidden h-5 shrink-0 text-lg font-semibold text-foreground md:block"
				><Logo /></a
			>
		{:else}
			<div
				in:fade={{ duration: 250, easing: cubicOut }}
				class="pointer-events-auto hidden shrink-0 items-center gap-2 md:flex"
			>
				<div class="h-[46px] {user ? 'w-56' : 'w-[46px]'} shrink-0" aria-hidden="true"></div>
			</div>
		{/if}
		{#if guestExplore}
			<a
				href={resolve('')}
				aria-label="Go to home"
				class="pointer-events-auto flex size-10 shrink-0 items-center justify-center rounded-full border border-border bg-primary-foreground/80 bg-clip-padding text-foreground shadow-sm backdrop-blur-sm md:hidden"
			>
				<span class="block size-8"><LogoIcon /></span>
			</a>
		{:else}
			<!-- This position matches the mobile save-detail home button. -->
			<a
				in:fade={{ duration: 250, easing: cubicOut }}
				href={resolve('')}
				aria-label="Go to home"
				class="pointer-events-auto fixed left-1/2 flex -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-transparent bg-primary-foreground/80 bg-clip-padding px-4 py-2.5 text-foreground shadow-sm backdrop-blur-sm md:hidden"
				style="top: calc(env(safe-area-inset-top) + 2rem)"
			>
				<span class="block h-5"><Logo /></span>
			</a>
		{/if}
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
		<div class="flex-1"></div>
		{#if user}
			<!-- The mobile bar carries its own actions. -->
			{@render desktopActions()}
		{:else if guestExplore}
			{@render guestActions(true)}
		{:else}
			<div class="hidden shrink-0 items-center gap-2 md:flex">
				{@render searchButton('glass', 'pointer-events-auto size-10 shrink-0', true)}
				<ThemeToggle class={floatingTheme} />
				{@render loginButton('lg')}
			</div>
		{/if}
	</header>
{/if}

<!-- Public Explore keeps the feed selector here on mobile. -->
{#if !landing && !mobileBottomBarState.hidden && (!guestExplore || showFlowField)}
	{#if activeSearch}
		<!-- The bottom cluster has no room beside the lens, so the active query rides
		     just above it (44px cluster + 0.5rem gap over the cluster's own offset). -->
		<button
			type="button"
			onclick={() => (searchCommandOpen = true)}
			class="fixed left-1/2 z-10 flex max-w-[calc(100vw-2rem)] -translate-x-1/2 items-baseline gap-1.5 rounded-full border border-transparent bg-primary-foreground/80 bg-clip-padding px-3 py-1.5 text-sm text-foreground shadow-sm backdrop-blur-sm md:hidden"
			style="bottom: calc(env(safe-area-inset-bottom) + 4.25rem)"
		>
			{@render searchQuery()}
		</button>
	{/if}
	<div
		transition:fly={{ y: 16, duration: 180, easing: cubicOut }}
		class="fixed left-1/2 z-10 -translate-x-1/2 md:hidden"
		style="bottom: calc(env(safe-area-inset-bottom) - {raisedMobileBar ? -0.5 : 1}rem)"
	>
		<div
			bind:this={bottomBarEl}
			class="{glassGroup} flex border-border p-1.5 shadow-md shadow-black/15 {user
				? '[&_[data-slot=avatar]]:size-10 [&_button]:size-11 [&_svg]:size-[1.375rem]'
				: ''}"
		>
			{#if user}
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
								{@render burgerIcon(openMenu === 'burger-mobile', true)}
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
				<ModeSwitcher
					mode={routeMode}
					variant="icon"
					side="top"
					anchor={bottomBarEl}
					bind:open={() => openMenu === 'mode-mobile', (v) => toggleMenu('mode-mobile', v)}
				/>
			{:else if !guestExplore}
				{@render loginButton('default')}
				<ThemeToggle
					class="h-9 gap-1 rounded-full bg-transparent pr-1 pl-4 text-foreground hover:bg-muted aria-expanded:bg-muted dark:hover:bg-muted/50"
				/>
			{/if}
			{#if !guestExplore}{@render searchButton('ghost', '', false)}{/if}
			<div
				inert={!showFlowField}
				aria-hidden={!showFlowField}
				class="flex h-11 shrink-0 items-center justify-center overflow-hidden transition-[width,opacity,transform] duration-200 ease-out {showFlowField
					? 'w-11 scale-100 opacity-100'
					: 'pointer-events-none w-0 scale-75 opacity-0'}"
			>
				<PersonalizationButton mobileBar />
			</div>
		</div>
	</div>
{/if}

<!-- ⌘K / Ctrl+K opens the search command from anywhere. -->
<svelte:window
	onkeydown={(e) => {
		if (actionsOnly) return;
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
	<NotificationsDialog bind:open={notificationsOpen} consentOutcome={moderationConsent} />
{/if}

<style>
	.explore-top-bar::after {
		content: '';
		position: absolute;
		inset: 0 0 -1rem;
		pointer-events: none;
		z-index: -1;
	}
</style>
