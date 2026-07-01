<script lang="ts">
	import { goto } from '$app/navigation';
	import { SvelteSet } from 'svelte/reactivity';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { apiFetch } from '$lib/api';
	import { isNative } from '$lib/platform';
	import { clearAuthToken } from '$lib/auth-storage';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import { favouriteCollections } from '$lib/stores/favourites.svelte';
	import type { CollectionView } from '$lib/types';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Input } from '$lib/components/ui/input';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import Logo from '$lib/assets/logo.svelte';
	import ListFilter from '@lucide/svelte/icons/list-filter';
	import ArrowDownUp from '@lucide/svelte/icons/arrow-down-up';
	import Folder from '@lucide/svelte/icons/folder';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Star from '@lucide/svelte/icons/star';
	import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down';
	import UserIcon from '@lucide/svelte/icons/user';
	import Settings from '@lucide/svelte/icons/settings';
	import LogOut from '@lucide/svelte/icons/log-out';

	let { selectedUri = '' }: { selectedUri?: string } = $props();

	let query = $state('');
	let q = $derived(query.trim().toLowerCase());

	// Sort order for collections, sections and favourites. Local UI preference.
	let sortMode = $state<'name' | 'recent'>('name');

	// Section headings collapse their whole group. Local UI preference.
	let libraryOpen = $state(true);
	let favouritesOpen = $state(true);

	function hrefFor(uri: string) {
		return uri ? `/organize?c=${encodeURIComponent(uri)}` : '/organize';
	}

	const byName = (a: CollectionView, b: CollectionView) =>
		a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
	// Most recent activity first (last save, falling back to creation), name as tiebreak.
	const byRecent = (a: CollectionView, b: CollectionView) => {
		const ta = a.lastSavedAt ?? a.createdAt ?? '';
		const tb = b.lastSavedAt ?? b.createdAt ?? '';
		if (ta === tb) return byName(a, b);
		return ta < tb ? 1 : -1;
	};
	let comparator = $derived(sortMode === 'recent' ? byRecent : byName);

	let roots = $derived(collections.items.filter((c) => !c.parentUri));
	let sectionsByParent = $derived.by(() => {
		const m = new Map<string, CollectionView[]>();
		for (const c of collections.items) {
			if (!c.parentUri) continue;
			const arr = m.get(c.parentUri) ?? [];
			arr.push(c);
			m.set(c.parentUri, arr);
		}
		for (const arr of m.values()) arr.sort(comparator);
		return m;
	});

	// Which roots are manually expanded (when not searching). Searching forces the
	// matching roots open and is handled in `tree` below.
	let openRoots = new SvelteSet<string>();

	// Auto-expand the root that holds the currently selected section (e.g. on a
	// deep link), so the active item is always visible in the tree.
	let selectedParentUri = $derived(
		collections.items.find((c) => c.uri === selectedUri)?.parentUri ?? null
	);

	type TreeNode = { root: CollectionView; sections: CollectionView[]; open: boolean };
	let tree = $derived.by<TreeNode[]>(() => {
		const sorted = [...roots].sort(comparator);
		if (!q) {
			return sorted.map((root) => ({
				root,
				sections: sectionsByParent.get(root.uri) ?? [],
				open: openRoots.has(root.uri) || root.uri === selectedParentUri
			}));
		}
		const out: TreeNode[] = [];
		for (const root of sorted) {
			const sections = sectionsByParent.get(root.uri) ?? [];
			const nameMatch = root.name.toLowerCase().includes(q);
			const matching = sections.filter((s) => s.name.toLowerCase().includes(q));
			if (nameMatch) out.push({ root, sections, open: true });
			else if (matching.length) out.push({ root, sections: matching, open: true });
		}
		return out;
	});

	let favourites = $derived.by(() => {
		const list = q
			? favouriteCollections.items.filter((c) => c.name.toLowerCase().includes(q))
			: favouriteCollections.items;
		return [...list].sort(comparator);
	});

	async function handleLogout() {
		if (isNative()) {
			try {
				await apiFetch('/oauth/logout');
			} catch {
				// best effort
			}
			await clearAuthToken();
			auth.user = null;
			auth.checked = true;
			goto('/');
		} else {
			window.location.href = `${PUBLIC_APPVIEW_URL}/oauth/logout`;
		}
	}

	let displayName = $derived(auth.user?.displayName || auth.user?.handle || '');
</script>

<Sidebar.Root collapsible="offcanvas" variant="inset" side="left">
	<Sidebar.Header class="gap-4 pb-1">
		<div class="flex items-center gap-2 px-1 pt-1">
			<a href="/organize" class="block h-5 text-foreground" aria-label="Currents">
				<Logo />
			</a>
			<span class="text-xs font-medium tracking-wide text-muted-foreground uppercase"
				>Secret organize mode</span
			>
		</div>
		<div class="flex items-center gap-2">
			<div class="relative flex-1">
				<ListFilter
					class="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground"
				/>
				<Input
					bind:value={query}
					placeholder="Filter collections…"
					class="h-9 bg-background pl-8"
					autocorrect="off"
					autocapitalize="off"
					spellcheck={false}
				/>
			</div>
			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button
							{...props}
							variant="ghost"
							size="icon"
							class="shrink-0"
							aria-label="Sort collections"
						>
							<ArrowDownUp />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end" class="w-44">
					<DropdownMenu.Label class="py-1.5">Sort by</DropdownMenu.Label>
					<DropdownMenu.RadioGroup
						value={sortMode}
						onValueChange={(v) => (sortMode = v as 'name' | 'recent')}
					>
						<DropdownMenu.RadioItem value="name">Alphabetical</DropdownMenu.RadioItem>
						<DropdownMenu.RadioItem value="recent">Recent activity</DropdownMenu.RadioItem>
					</DropdownMenu.RadioGroup>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		</div>
	</Sidebar.Header>

	<Sidebar.Content>
		<Collapsible.Root bind:open={libraryOpen} class="group/section">
			<Sidebar.Group>
				<Sidebar.GroupLabel
					class="sticky top-0 z-10 cursor-pointer rounded-none bg-sidebar hover:text-foreground"
				>
					{#snippet child({ props })}
						<Collapsible.Trigger {...props}>
							My library
							<ChevronRight
								class="ml-auto size-4 shrink-0 transition-transform group-data-[state=open]/section:rotate-90"
							/>
						</Collapsible.Trigger>
					{/snippet}
				</Sidebar.GroupLabel>
				<Collapsible.Content>
					<Sidebar.GroupContent>
						<Sidebar.Menu>
							{#each tree as node (node.root.uri)}
								<Collapsible.Root
									class="group/collapsible"
									open={node.open}
									onOpenChange={(o) => {
										if (o) openRoots.add(node.root.uri);
										else openRoots.delete(node.root.uri);
									}}
								>
									<Sidebar.MenuItem>
										<Sidebar.MenuButton isActive={selectedUri === node.root.uri} class="h-8">
											{#snippet child({ props })}
												<a href={hrefFor(node.root.uri)} {...props}>
													<Folder />
													<span>{node.root.name}</span>
												</a>
											{/snippet}
										</Sidebar.MenuButton>
										{#if node.sections.length > 0}
											<Collapsible.Trigger
												data-sidebar="menu-action"
												class="absolute top-0 right-1 flex h-8 w-7 items-center justify-center rounded-md text-sidebar-foreground/70 outline-none hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
											>
												<ChevronRight
													class="size-4 transition-transform group-data-[state=open]/collapsible:rotate-90"
												/>
												<span class="sr-only">Toggle sections</span>
											</Collapsible.Trigger>
											<Collapsible.Content>
												<Sidebar.MenuSub>
													{#each node.sections as section (section.uri)}
														<Sidebar.MenuSubItem>
															<Sidebar.MenuSubButton
																href={hrefFor(section.uri)}
																isActive={selectedUri === section.uri}
															>
																<span>{section.name}</span>
															</Sidebar.MenuSubButton>
														</Sidebar.MenuSubItem>
													{/each}
												</Sidebar.MenuSub>
											</Collapsible.Content>
										{/if}
									</Sidebar.MenuItem>
								</Collapsible.Root>
							{/each}
							{#if tree.length === 0}
								{#if !collections.loaded}
									<div class="flex items-center gap-2 px-2 py-1.5 text-xs text-muted-foreground">
										<Spinner class="size-3.5" />
										Loading…
									</div>
								{:else}
									<p class="px-2 py-1.5 text-xs text-muted-foreground">
										{q ? 'No collections match.' : 'No collections yet.'}
									</p>
								{/if}
							{/if}
						</Sidebar.Menu>
					</Sidebar.GroupContent>
				</Collapsible.Content>
			</Sidebar.Group>
		</Collapsible.Root>

		{#if !favouriteCollections.loaded || favourites.length > 0}
			<Collapsible.Root bind:open={favouritesOpen} class="group/section">
				<Sidebar.Group>
					<Sidebar.GroupLabel
						class="sticky top-0 z-10 cursor-pointer rounded-none bg-sidebar hover:text-foreground"
					>
						{#snippet child({ props })}
							<Collapsible.Trigger {...props}>
								Favourites from others
								<ChevronRight
									class="ml-auto size-4 shrink-0 transition-transform group-data-[state=open]/section:rotate-90"
								/>
							</Collapsible.Trigger>
						{/snippet}
					</Sidebar.GroupLabel>
					<Collapsible.Content>
						<Sidebar.GroupContent>
							<Sidebar.Menu>
								{#each favourites as fav (fav.uri)}
									<Sidebar.MenuItem>
										<Sidebar.MenuButton isActive={selectedUri === fav.uri} class="h-8">
											{#snippet child({ props })}
												<a href={hrefFor(fav.uri)} {...props}>
													<Star />
													<span>{fav.name}</span>
												</a>
											{/snippet}
										</Sidebar.MenuButton>
									</Sidebar.MenuItem>
								{/each}
								{#if favourites.length === 0 && !favouriteCollections.loaded}
									<div class="flex items-center gap-2 px-2 py-1.5 text-xs text-muted-foreground">
										<Spinner class="size-3.5" />
										Loading…
									</div>
								{/if}
							</Sidebar.Menu>
						</Sidebar.GroupContent>
					</Collapsible.Content>
				</Sidebar.Group>
			</Collapsible.Root>
		{/if}
	</Sidebar.Content>

	<Sidebar.Footer>
		<Sidebar.Menu>
			<Sidebar.MenuItem>
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Sidebar.MenuButton
								{...props}
								size="lg"
								class="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
							>
								<Avatar.Root class="size-8 rounded-lg">
									{#if auth.user?.avatar}
										<Avatar.Image src={auth.user.avatar} alt={displayName} />
									{/if}
									<Avatar.Fallback class="rounded-lg">
										<UserIcon class="size-4" />
									</Avatar.Fallback>
								</Avatar.Root>
								<div class="grid flex-1 text-left text-sm leading-tight">
									<span class="truncate font-medium">{displayName}</span>
									{#if auth.user}
										<span class="truncate text-xs text-muted-foreground">@{auth.user.handle}</span>
									{/if}
								</div>
								<ChevronsUpDown class="ml-auto size-4" />
							</Sidebar.MenuButton>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content side="top" align="start" class="w-56">
						{#if auth.user}
							<DropdownMenu.Item onclick={() => goto(`/profile/${auth.user!.handle}`)}>
								<UserIcon class="size-4" />
								Profile
							</DropdownMenu.Item>
						{/if}
						<DropdownMenu.Item onclick={() => goto('/settings')}>
							<Settings class="size-4" />
							Settings
						</DropdownMenu.Item>
						<DropdownMenu.Separator />
						<DropdownMenu.Item onclick={handleLogout}>
							<LogOut class="size-4" />
							Log out
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			</Sidebar.MenuItem>
		</Sidebar.Menu>
	</Sidebar.Footer>
</Sidebar.Root>
