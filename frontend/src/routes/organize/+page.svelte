<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetch } from '$lib/api';
	import { page } from '$app/state';
	import { SvelteSet } from 'svelte/reactivity';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import * as Breadcrumb from '$lib/components/ui/breadcrumb';
	import * as Popover from '$lib/components/ui/popover';
	import * as Command from '$lib/components/ui/command';
	import { Separator } from '$lib/components/ui/separator';
	import { Button } from '$lib/components/ui/button';
	import OrganizeSidebarLeft from '$lib/components/organize/sidebar-left.svelte';
	import OrganizeCanvas from '$lib/components/organize/canvas.svelte';
	import OrganizeSidebarRight from '$lib/components/organize/sidebar-right.svelte';
	import OrganizeSearchCommand from '$lib/components/organize/search-command.svelte';
	import CollectionFilterItems from '$lib/components/organize/collection-filter-items.svelte';
	import { Kbd } from '$lib/components/ui/kbd';
	import { collections } from '$lib/stores/collections.svelte';
	import { favouriteCollections } from '$lib/stores/favourites.svelte';
	import { requireSupporter, requireColorSearch } from '$lib/stores/supporter.svelte';
	import { getImageContent, type SaveView } from '$lib/types';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import SearchIcon from '@lucide/svelte/icons/search';
	import Sparkles from '@lucide/svelte/icons/sparkles';
	import ListFilter from '@lucide/svelte/icons/list-filter';
	import X from '@lucide/svelte/icons/x';

	// The selected collection/section lives in the URL (`?c=<uri>`), so tree rows are real
	// links. Find-similar lives in `?sim=<sourceUri>` so it gets its own history entry —
	// back returns to the prior view. Both empty = the "My library" root.
	let selectedUri = $derived(page.url.searchParams.get('c') ?? '');
	let similarUri = $derived(page.url.searchParams.get('sim') ?? '');

	// Text (ephemeral `searchQuery`) and color (URL `?color=`, below) can combine
	// into a hybrid search: the color filters, the text orders. They share one
	// live-editable collection `scope`. Find-similar (`?sim=`) is exclusive with
	// both. Navigating to a collection clears the text query and scope; the
	// URL-driven ?color / ?sim clear themselves on navigation.
	let searchOpen = $state(false);
	let searchQuery = $state<string | null>(null);
	let scope = new SvelteSet<string>();
	let search = $derived(searchQuery ? { query: searchQuery, collections: [...scope] } : null);
	$effect(() => {
		void selectedUri;
		untrack(() => {
			searchQuery = null;
			scope.clear();
		});
	});
	function toggleScope(uri: string) {
		if (scope.has(uri)) scope.delete(uri);
		else scope.add(uri);
	}
	let scopeLabel = $derived(
		scope.size === 0 ? 'Whole library' : `${scope.size} collection${scope.size === 1 ? '' : 's'}`
	);

	// The source save (for the chip thumbnail) and the collection scope for find-similar,
	// both ephemeral; the scope resets whenever the source changes.
	let similarSource = $state<SaveView | null>(null);
	let similarScope = new SvelteSet<string>();
	let prevSimUri = '';
	$effect(() => {
		void similarUri;
		untrack(() => {
			if (similarUri !== prevSimUri) {
				similarScope.clear();
				prevSimUri = similarUri;
			}
		});
	});
	let similarImage = $derived(
		similarSource && similarSource.uri === similarUri ? getImageContent(similarSource) : null
	);

	// A hard load into a `?sim=` URL has no stashed source save, which would leave
	// the header chip without its thumbnail (and nothing to reopen details from) —
	// refetch it by uri.
	$effect(() => {
		const uri = similarUri;
		if (!uri || similarSource?.uri === uri) return;
		let stale = false;
		(async () => {
			try {
				const res = await apiFetch(
					`/xrpc/is.currents.feed.getSaves?uris=${encodeURIComponent(uri)}`
				);
				if (!res.ok) return;
				const save = ((await res.json()).saves ?? [])[0] as SaveView | undefined;
				if (save && !stale) similarSource = save;
			} catch {
				// the thumbnail simply stays hidden
			}
		})();
		return () => {
			stale = true;
		};
	});
	let similar = $derived(similarUri ? { uri: similarUri, collections: [...similarScope] } : null);

	function simHref(uri: string) {
		const p = new URLSearchParams();
		if (selectedUri) p.set('c', selectedUri);
		p.set('sim', uri);
		return `/organize?${p}`;
	}
	function toggleSimScope(uri: string) {
		if (similarScope.has(uri)) similarScope.delete(uri);
		else similarScope.add(uri);
	}

	// Color search lives in the URL (`?color=<hex>`, no leading #) for its own
	// history entry. It shares `scope` and combines with `searchQuery` into a
	// hybrid search.
	let colorParam = $derived(page.url.searchParams.get('color') ?? '');
	let colorSearch = $derived(
		colorParam ? { hex: '#' + colorParam, collections: [...scope] } : null
	);
	function colorHref(hex: string) {
		const p = new URLSearchParams();
		if (selectedUri) p.set('c', selectedUri);
		p.set('color', hex.replace('#', '').toLowerCase());
		return `/organize?${p}`;
	}

	// Resolve against own + favourited collections (a selection can be either).
	let known = $derived([...collections.items, ...favouriteCollections.items]);
	let selected = $derived(known.find((c) => c.uri === selectedUri) ?? null);
	let parent = $derived(
		selected?.parentUri ? (known.find((c) => c.uri === selected!.parentUri) ?? null) : null
	);

	function hrefFor(uri: string) {
		return uri ? `/organize?c=${encodeURIComponent(uri)}` : '/organize';
	}

	// Library search and find-similar are supporter-tier features (enforced
	// server-side too); requireSupporter opens the shared paywall for
	// non-supporters. Color search runs through requireColorSearch instead —
	// same paywall, but only once the free trial colors are used up. The
	// paywall itself is mounted once in the root layout.
	async function findSimilar(s: SaveView) {
		if (!(await requireSupporter(() => void findSimilar(s)))) return;
		searchQuery = null;
		scope.clear();
		similarSource = s;
		// On mobile the detail panel is a full-screen overlay that would cover the
		// results, so close it; the header chip's thumbnail reopens it on demand.
		if (isMobile.current) selection = null;
		// simHref carries no ?color, so navigating here also clears any color search.
		goto(simHref(s.uri));
	}

	// Color search in the library, optionally with a text query (hybrid: the color
	// filters, the text orders). Navigates to a `?color=` URL (dropping any
	// find-similar); the ephemeral text query survives the same-collection nav.
	async function searchColorInLibrary(hex: string, text?: string) {
		if (!(await requireColorSearch(() => void searchColorInLibrary(hex, text)))) return;
		searchQuery = text?.trim() ? text.trim() : null;
		scope.clear();
		if (isMobile.current) selection = null;
		goto(colorHref(hex));
	}
	async function onColorSearch(hex: string, where: 'explore' | 'library') {
		if (where === 'library') return searchColorInLibrary(hex);
		const q = hex.replace('#', '').toLowerCase();
		const go = () =>
			goto(resolve('/(with-navbar)/search/[type]/[query]', { type: 'color', query: q }));
		if (await requireColorSearch(go)) go();
	}

	// ⌘K / Ctrl+K opens the library search from anywhere in organize mode.
	function onWindowKeydown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
			e.preventDefault();
			searchOpen = true;
		}
	}
	const isMac =
		typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform ?? '');

	// The image detail panel opens on tile click. The selection is scoped to the
	// collection it was made in, so switching collections closes the panel for free
	// (selectedSave derives to null once the collection no longer matches).
	let selection = $state<{ collectionUri: string; save: SaveView } | null>(null);
	let selectedSave = $derived(
		selection && selection.collectionUri === selectedUri ? selection.save : null
	);
	const isMobile = new IsMobile();
</script>

<svelte:head>
	<title>{selected ? selected.name + ' · Organize · Currents' : 'Organize · Currents'}</title>
</svelte:head>

<svelte:window onkeydown={onWindowKeydown} />

<!-- The shared collection-scope filter used by the text, color, and hybrid chips
     (find-similar keeps its own, on similarScope). -->
{#snippet scopeFilter()}
	<Popover.Root>
		<Popover.Trigger>
			{#snippet child({ props })}
				<Button
					{...props}
					variant="ghost"
					size="icon-sm"
					class="relative"
					aria-label="Filter by collection"
				>
					<ListFilter />
					{#if scope.size > 0}
						<span
							class="absolute -top-0.5 -right-0.5 flex size-3.5 items-center justify-center rounded-full bg-primary text-[10px] leading-none font-semibold text-primary-foreground"
						>
							{scope.size}
						</span>
					{/if}
				</Button>
			{/snippet}
		</Popover.Trigger>
		<Popover.Content align="start" class="w-64 p-0">
			<Command.Root shouldFilter={false} class="bg-transparent">
				<Command.List>
					<CollectionFilterItems
						collections={collections.items}
						favourites={favouriteCollections.items}
						selected={scope}
						onToggle={toggleScope}
					/>
				</Command.List>
			</Command.Root>
		</Popover.Content>
	</Popover.Root>
{/snippet}

<!-- Bound the shell to the viewport (the wrapper is min-h-svh by default) so the
     central canvas and right panel scroll internally instead of the whole page.
     Use dvh, not svh: with internal scrolling the mobile URL bar stays retracted,
     so an svh shell falls short of the visible viewport and leaks a strip of body
     background at the bottom. dvh tracks the live viewport (and can't jitter here,
     since the page itself never scrolls). -->
<Sidebar.Provider class="h-dvh overflow-hidden">
	<OrganizeSidebarLeft {selectedUri} />
	<Sidebar.Inset class="overflow-hidden">
		<header
			class="flex h-[calc(3.5rem+env(safe-area-inset-top))] shrink-0 items-center gap-2 border-b px-4 pt-[env(safe-area-inset-top)]"
		>
			<Sidebar.Trigger class="-ml-1" />
			<Separator orientation="vertical" class="mr-1 data-[orientation=vertical]:h-4" />
			{#if search && colorSearch}
				<!-- Hybrid: the color filters, the text orders. The chip shows the color
				     (click to adjust it), the text, the shared scope filter, and a clear-all. -->
				<div class="flex min-w-0 items-center gap-2 text-sm font-medium">
					<button
						type="button"
						onclick={() => (searchOpen = true)}
						class="size-4 shrink-0 rounded-full border transition-transform hover:scale-110"
						style="background-color: {colorSearch.hex}"
						aria-label="Change color"
						title="Change color"
					></button>
					<span class="min-w-0 truncate">“{search.query}”</span>
					{@render scopeFilter()}
					<button
						type="button"
						onclick={() => {
							searchQuery = null;
							goto(hrefFor(selectedUri));
						}}
						class="rounded p-1 text-muted-foreground hover:bg-muted"
						aria-label="Clear search"
					>
						<X class="size-3.5" />
					</button>
				</div>
			{:else if search}
				<!-- A text search replaces the breadcrumb with this chip: it names the mode
				     and shows the collection scope in words. The scope is live-editable via
				     the filter (re-running the search without reopening the command); the
				     query text and clear control live in the search field on the right. -->
				<div class="flex min-w-0 items-center gap-2 text-sm font-medium">
					<SearchIcon class="size-4 shrink-0 text-muted-foreground" />
					<span class="truncate">Search results</span>
					<span class="shrink-0 text-muted-foreground">·</span>
					<span class="shrink-0 text-muted-foreground">{scopeLabel}</span>
					<Popover.Root>
						<Popover.Trigger>
							{#snippet child({ props })}
								<Button {...props} variant="ghost" size="icon-sm" aria-label="Filter by collection">
									<ListFilter />
								</Button>
							{/snippet}
						</Popover.Trigger>
						<Popover.Content align="start" class="w-64 p-0">
							<Command.Root shouldFilter={false} class="bg-transparent">
								<Command.List>
									<CollectionFilterItems
										collections={collections.items}
										favourites={favouriteCollections.items}
										selected={scope}
										onToggle={toggleScope}
									/>
								</Command.List>
							</Command.Root>
						</Popover.Content>
					</Popover.Root>
					<!-- On mobile the search field (and its clear X) collapses to an icon,
					     so the chip carries the clear control instead. -->
					<button
						type="button"
						onclick={() => (searchQuery = null)}
						class="rounded p-1 text-muted-foreground hover:bg-muted md:hidden"
						aria-label="Clear search"
					>
						<X class="size-3.5" />
					</button>
				</div>
			{:else if similarUri}
				<!-- Find-similar overlays the grid with visually similar images; this chip
				     (in place of the breadcrumb) shows the source, a collection filter, and
				     a clear button that navigates back to the underlying view. -->
				<div class="flex min-w-0 items-center gap-2 text-sm font-medium">
					<Sparkles class="size-4 shrink-0 text-muted-foreground" />
					{#if similarImage && similarSource}
						{@const source = similarSource}
						<button
							type="button"
							class="shrink-0 rounded transition-opacity hover:opacity-75"
							onclick={() => (selection = { collectionUri: selectedUri, save: source })}
							aria-label="Show image details"
						>
							<img src={similarImage.imageUrl} alt="" class="size-6 rounded object-cover" />
						</button>
					{/if}
					<span class="truncate">Similar images</span>
					<Popover.Root>
						<Popover.Trigger>
							{#snippet child({ props })}
								<Button
									{...props}
									variant="ghost"
									size="icon-sm"
									class="relative"
									aria-label="Filter by collection"
								>
									<ListFilter />
									{#if similarScope.size > 0}
										<span
											class="absolute -top-0.5 -right-0.5 flex size-3.5 items-center justify-center rounded-full bg-primary text-[10px] leading-none font-semibold text-primary-foreground"
										>
											{similarScope.size}
										</span>
									{/if}
								</Button>
							{/snippet}
						</Popover.Trigger>
						<Popover.Content align="start" class="w-64 p-0">
							<Command.Root shouldFilter={false} class="bg-transparent">
								<Command.List>
									<CollectionFilterItems
										collections={collections.items}
										favourites={favouriteCollections.items}
										selected={similarScope}
										onToggle={toggleSimScope}
									/>
								</Command.List>
							</Command.Root>
						</Popover.Content>
					</Popover.Root>
					<button
						type="button"
						onclick={() => goto(hrefFor(selectedUri))}
						class="rounded p-1 text-muted-foreground hover:bg-muted"
						aria-label="Clear find similar"
					>
						<X class="size-3.5" />
					</button>
				</div>
			{:else if colorSearch}
				<!-- Color search overlays the grid with library images matching a color;
				     this chip shows the color (click to adjust it), a collection filter,
				     and a clear button. -->
				<div class="flex min-w-0 items-center gap-2 text-sm font-medium">
					<button
						type="button"
						onclick={() => (searchOpen = true)}
						class="size-4 shrink-0 rounded-full border transition-transform hover:scale-110"
						style="background-color: {colorSearch.hex}"
						aria-label="Change color"
						title="Change color"
					></button>
					<span class="truncate">Color search</span>
					<Popover.Root>
						<Popover.Trigger>
							{#snippet child({ props })}
								<Button
									{...props}
									variant="ghost"
									size="icon-sm"
									class="relative"
									aria-label="Filter by collection"
								>
									<ListFilter />
									{#if scope.size > 0}
										<span
											class="absolute -top-0.5 -right-0.5 flex size-3.5 items-center justify-center rounded-full bg-primary text-[10px] leading-none font-semibold text-primary-foreground"
										>
											{scope.size}
										</span>
									{/if}
								</Button>
							{/snippet}
						</Popover.Trigger>
						<Popover.Content align="start" class="w-64 p-0">
							<Command.Root shouldFilter={false} class="bg-transparent">
								<Command.List>
									<CollectionFilterItems
										collections={collections.items}
										favourites={favouriteCollections.items}
										selected={scope}
										onToggle={toggleScope}
									/>
								</Command.List>
							</Command.Root>
						</Popover.Content>
					</Popover.Root>
					<button
						type="button"
						onclick={() => goto(hrefFor(selectedUri))}
						class="rounded p-1 text-muted-foreground hover:bg-muted"
						aria-label="Clear color search"
					>
						<X class="size-3.5" />
					</button>
				</div>
			{:else}
				<Breadcrumb.Root>
					<Breadcrumb.List>
						<Breadcrumb.Item>
							{#if selected}
								<Breadcrumb.Link href="/organize">My library</Breadcrumb.Link>
							{:else}
								<Breadcrumb.Page>My library</Breadcrumb.Page>
							{/if}
						</Breadcrumb.Item>
						{#if parent}
							<Breadcrumb.Separator />
							<Breadcrumb.Item>
								<Breadcrumb.Link href={hrefFor(parent.uri)}>{parent.name}</Breadcrumb.Link>
							</Breadcrumb.Item>
						{/if}
						{#if selected}
							<Breadcrumb.Separator />
							<Breadcrumb.Item>
								<Breadcrumb.Page>{selected.name}</Breadcrumb.Page>
							</Breadcrumb.Item>
						{/if}
					</Breadcrumb.List>
				</Breadcrumb.Root>
			{/if}

			<!-- Search: icon-only on mobile (the field would crowd the header); on desktop
			     a field whose text area opens the command dialog and whose X clears an
			     active search and restores the collection grid. -->
			<Button
				variant="ghost"
				size="icon"
				class="ml-auto shrink-0 md:hidden"
				aria-label="Search your library"
				onclick={() => (searchOpen = true)}
			>
				<SearchIcon />
			</Button>
			<div
				class="ml-auto hidden h-9 w-full max-w-64 items-center rounded-md border bg-background text-sm md:flex"
			>
				<button
					type="button"
					onclick={() => (searchOpen = true)}
					class="flex min-w-0 flex-1 items-center gap-2 px-3 text-muted-foreground hover:text-foreground"
				>
					<SearchIcon class="size-4 shrink-0" />
					<span class="truncate">{search ? search.query : 'Search your library…'}</span>
					{#if !search}
						<Kbd class="ml-auto hidden shrink-0 md:inline-flex">{isMac ? '⌘' : 'Ctrl'} K</Kbd>
					{/if}
				</button>
				{#if search}
					<button
						type="button"
						onclick={() => (searchQuery = null)}
						class="mr-1 rounded p-1 text-muted-foreground hover:bg-muted"
						aria-label="Clear search"
					>
						<X class="size-3.5" />
					</button>
				{/if}
			</div>
		</header>

		<OrganizeCanvas
			{selectedUri}
			{search}
			{similar}
			color={colorSearch}
			selectedSaveUri={selectedSave?.uri ?? null}
			onSelectSave={(s) => (selection = { collectionUri: selectedUri, save: s })}
			onFindSimilar={findSimilar}
		/>
	</Sidebar.Inset>

	{#if selectedSave}
		<OrganizeSidebarRight
			save={selectedSave}
			onClose={() => (selection = null)}
			onSavesChange={(saves) => {
				if (selection) selection.save.viewer = { ...(selection.save.viewer ?? {}), saves };
			}}
			onFindSimilar={findSimilar}
			{onColorSearch}
		/>
	{/if}

	<OrganizeSearchCommand
		bind:open={searchOpen}
		collections={collections.items}
		favourites={favouriteCollections.items}
		initial={selectedUri ? [selectedUri] : []}
		initialText={searchQuery ?? ''}
		canSearch={() => requireSupporter(() => (searchOpen = true))}
		onSearch={(q, cols) => {
			// Text search is ephemeral; drop any find-similar or color search from the URL.
			if (similarUri || colorParam) goto(hrefFor(selectedUri));
			scope.clear();
			if (q) for (const c of cols) scope.add(c);
			searchQuery = q || null;
		}}
		onColorSearch={(hex, text) => searchColorInLibrary(hex, text)}
	/>
</Sidebar.Provider>
