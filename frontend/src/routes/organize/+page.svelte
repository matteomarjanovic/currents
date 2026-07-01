<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
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
	import { collections } from '$lib/stores/collections.svelte';
	import { favouriteCollections } from '$lib/stores/favourites.svelte';
	import { getImageContent, type SaveView } from '$lib/types';
	import SearchIcon from '@lucide/svelte/icons/search';
	import Sparkles from '@lucide/svelte/icons/sparkles';
	import ListFilter from '@lucide/svelte/icons/list-filter';
	import X from '@lucide/svelte/icons/x';

	// The selected collection/section lives in the URL (`?c=<uri>`), so tree rows are real
	// links. Find-similar lives in `?sim=<sourceUri>` so it gets its own history entry —
	// back returns to the prior view. Both empty = the "My library" root.
	let selectedUri = $derived(page.url.searchParams.get('c') ?? '');
	let similarUri = $derived(page.url.searchParams.get('sim') ?? '');

	// A text search (ephemeral) is mutually exclusive with find-similar. Navigating to a
	// collection clears the search; find-similar is URL-driven so it clears itself. The
	// query and a live-editable collection scope combine into `search`; toggling the scope
	// from the header chip re-runs the search without reopening the command.
	let searchOpen = $state(false);
	let searchQuery = $state<string | null>(null);
	let searchScope = new SvelteSet<string>();
	let search = $derived(searchQuery ? { query: searchQuery, collections: [...searchScope] } : null);
	$effect(() => {
		void selectedUri;
		untrack(() => (searchQuery = null));
	});
	function toggleSearchScope(uri: string) {
		if (searchScope.has(uri)) searchScope.delete(uri);
		else searchScope.add(uri);
	}
	let searchScopeLabel = $derived(
		searchScope.size === 0
			? 'Whole library'
			: `${searchScope.size} collection${searchScope.size === 1 ? '' : 's'}`
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

	// Resolve against own + favourited collections (a selection can be either).
	let known = $derived([...collections.items, ...favouriteCollections.items]);
	let selected = $derived(known.find((c) => c.uri === selectedUri) ?? null);
	let parent = $derived(
		selected?.parentUri ? (known.find((c) => c.uri === selected!.parentUri) ?? null) : null
	);

	function hrefFor(uri: string) {
		return uri ? `/organize?c=${encodeURIComponent(uri)}` : '/organize';
	}

	// The image detail panel opens on tile click. The selection is scoped to the
	// collection it was made in, so switching collections closes the panel for free
	// (selectedSave derives to null once the collection no longer matches).
	let selection = $state<{ collectionUri: string; save: SaveView } | null>(null);
	let selectedSave = $derived(
		selection && selection.collectionUri === selectedUri ? selection.save : null
	);
</script>

<!-- Bound the shell to the viewport (the wrapper is min-h-svh by default) so the
     central canvas and right panel scroll internally instead of the whole page. -->
<Sidebar.Provider class="h-svh overflow-hidden">
	<OrganizeSidebarLeft {selectedUri} />
	<Sidebar.Inset class="overflow-hidden">
		<header class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
			<Sidebar.Trigger class="-ml-1" />
			<Separator orientation="vertical" class="mr-1 data-[orientation=vertical]:h-4" />
			{#if search}
				<!-- A text search replaces the breadcrumb with this chip: it names the mode
				     and shows the collection scope in words. The scope is live-editable via
				     the filter (re-running the search without reopening the command); the
				     query text and clear control live in the search field on the right. -->
				<div class="flex min-w-0 items-center gap-2 text-sm font-medium">
					<SearchIcon class="size-4 shrink-0 text-muted-foreground" />
					<span class="truncate">Search results</span>
					<span class="shrink-0 text-muted-foreground">·</span>
					<span class="shrink-0 text-muted-foreground">{searchScopeLabel}</span>
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
										selected={searchScope}
										onToggle={toggleSearchScope}
									/>
								</Command.List>
							</Command.Root>
						</Popover.Content>
					</Popover.Root>
				</div>
			{:else if similarUri}
				<!-- Find-similar overlays the grid with visually similar images; this chip
				     (in place of the breadcrumb) shows the source, a collection filter, and
				     a clear button that navigates back to the underlying view. -->
				<div class="flex min-w-0 items-center gap-2 text-sm font-medium">
					<Sparkles class="size-4 shrink-0 text-muted-foreground" />
					{#if similarImage}
						<img src={similarImage.imageUrl} alt="" class="size-6 shrink-0 rounded object-cover" />
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

			<!-- Search field: the text area opens the command dialog; the X clears an
			     active search and restores the collection grid. -->
			<div
				class="ml-auto flex h-9 w-full max-w-64 items-center rounded-md border bg-background text-sm"
			>
				<button
					type="button"
					onclick={() => (searchOpen = true)}
					class="flex min-w-0 flex-1 items-center gap-2 px-3 text-muted-foreground hover:text-foreground"
				>
					<SearchIcon class="size-4 shrink-0" />
					<span class="truncate">{search ? search.query : 'Search your library…'}</span>
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
			selectedSaveUri={selectedSave?.uri ?? null}
			onSelectSave={(s) => (selection = { collectionUri: selectedUri, save: s })}
			onFindSimilar={(s) => {
				searchQuery = null;
				similarSource = s;
				goto(simHref(s.uri));
			}}
		/>
	</Sidebar.Inset>

	{#if selectedSave}
		<OrganizeSidebarRight
			save={selectedSave}
			onClose={() => (selection = null)}
			onSavesChange={(saves) => {
				if (selection) selection.save.viewer = { ...(selection.save.viewer ?? {}), saves };
			}}
			onFindSimilar={(s) => {
				searchQuery = null;
				similarSource = s;
				goto(simHref(s.uri));
			}}
		/>
	{/if}

	<OrganizeSearchCommand
		bind:open={searchOpen}
		collections={collections.items}
		favourites={favouriteCollections.items}
		initial={selectedUri ? [selectedUri] : []}
		onSearch={(q, cols) => {
			if (similarUri) goto(hrefFor(selectedUri));
			searchScope.clear();
			if (q) for (const c of cols) searchScope.add(c);
			searchQuery = q || null;
		}}
	/>
</Sidebar.Provider>
