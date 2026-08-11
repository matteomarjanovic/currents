<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { slide } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import { SvelteSet } from 'svelte/reactivity';
	import { apiFetch } from '$lib/api';
	import { type SaveView } from '$lib/types';
	import { distinctBlobCids, saveRkeys } from '$lib/organize-bulk';
	import { downloadImage } from '$lib/save-actions';
	import { useSidebar } from '$lib/components/ui/sidebar';
	import { Button } from '$lib/components/ui/button';
	import * as Popover from '$lib/components/ui/popover';
	import * as Drawer from '$lib/components/ui/drawer';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import BulkAttributionDialog from '$lib/components/bulk-attribution-dialog.svelte';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import FolderInput from '@lucide/svelte/icons/folder-input';
	import Download from '@lucide/svelte/icons/download';
	import Tag from '@lucide/svelte/icons/tag';
	import Quote from '@lucide/svelte/icons/quote';
	import X from '@lucide/svelte/icons/x';
	import ListChecks from '@lucide/svelte/icons/list-checks';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';

	let {
		saves,
		selectableCount,
		canMove,
		ownContext,
		onSelectAll,
		onClear,
		onExit,
		onCopy,
		onMove
	}: {
		saves: SaveView[];
		selectableCount: number;
		canMove: boolean;
		ownContext: boolean;
		onSelectAll: () => void;
		onClear: () => void;
		onExit: () => void;
		onCopy: (dest: string) => void;
		onMove: (dest: string) => void;
	} = $props();

	const sidebar = useSidebar();

	// Add-only self-label vocab (mirrors the server's allowed set).
	const SELF_LABEL_OPTIONS = [
		{ val: 'porn', label: 'Porn' },
		{ val: 'sexual', label: 'Sexual' },
		{ val: 'nudity', label: 'Nudity' },
		{ val: 'graphic-media', label: 'Graphic' },
		{ val: 'currents-ai-generated', label: 'AI-generated' }
	];

	// Copy/Move: two popovers on desktop, each anchored to its own button.
	let copyOpen = $state(false);
	let moveOpen = $state(false);

	// Mobile has no room for a row of labelled buttons, so it gets a floating pill
	// instead of the bar and everything lives one tap deeper. Crucially this is ONE
	// drawer whose view swaps — Copy/Move/Labels each need a surface of their own,
	// and opening a second drawer on top of the menu is what leaks the body
	// scroll-lock (see e2e/scroll-lock.spec.ts).
	type MobileView = 'menu' | 'copy' | 'move' | 'labels';
	let mobileView = $state<MobileView | null>(null);

	function pick(dest: string) {
		if (mobileView === 'move') onMove(dest);
		else if (mobileView === 'copy') onCopy(dest);
		mobileView = null;
	}

	let attributionOpen = $state(false);
	let blobCids = $derived(distinctBlobCids(saves));

	// Labels: chip toggles + Apply, in a popover (desktop) / drawer (mobile).
	let labelsOpen = $state(false);
	let chosen = new SvelteSet<string>();
	let applying = $state(false);
	function toggleLabel(val: string) {
		if (chosen.has(val)) chosen.delete(val);
		else chosen.add(val);
	}
	async function applyLabels() {
		const rkeys = saveRkeys(saves);
		const labels = [...chosen];
		if (rkeys.length === 0 || labels.length === 0) return;
		applying = true;
		try {
			const res = await apiFetch(`/save/labels/bulk`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ rkeys, labels })
			});
			if (!res.ok) {
				toast.error(`Could not apply labels (${res.status}).`);
				return;
			}
			const r = (await res.json()) as { applied: number; skipped: number; failed: number };
			toast.success(
				`Labels applied to ${r.applied}${r.skipped ? ` · ${r.skipped} skipped` : ''}${r.failed ? ` · ${r.failed} failed` : ''}`
			);
			labelsOpen = false;
			mobileView = null;
			chosen.clear();
			onExit();
		} catch {
			toast.error('Network error. Please try again.');
		} finally {
			applying = false;
		}
	}

	async function downloadAll() {
		// Sequential with a small gap: browsers drop rapid-fire programmatic downloads.
		for (const s of saves) {
			await downloadImage(s);
			await new Promise((r) => setTimeout(r, 250));
		}
	}

	// Attribution is a shared Dialog (desktop opens it straight from the bar), so on
	// mobile it's the one action that can't become a drawer view. Let the drawer
	// finish closing before the dialog mounts — overlapping the two fights over the
	// body scroll-lock and can leave the page unscrollable after both are dismissed.
	function openAttributionFromMenu() {
		mobileView = null;
		setTimeout(() => (attributionOpen = true), 250);
	}
</script>

<!-- The destination list, reused by the copy/move popovers and the mobile drawer. -->
{#snippet destinationList(onSelect: (uri: string) => void)}
	<div class="max-h-[50vh] overflow-y-auto p-1.5">
		<CollectionSelector variant="inline" {onSelect} />
	</div>
{/snippet}

{#snippet labelBody()}
	<div class="flex flex-wrap gap-1.5 p-1">
		{#each SELF_LABEL_OPTIONS as opt (opt.val)}
			{@const active = chosen.has(opt.val)}
			<button
				type="button"
				onclick={() => toggleLabel(opt.val)}
				disabled={applying}
				class="rounded-full border px-2.5 py-1 text-xs transition-colors {active
					? 'border-foreground bg-foreground text-background'
					: 'border-border text-muted-foreground hover:bg-muted'}"
			>
				{opt.label}
			</button>
		{/each}
	</div>
	<p class="px-1 pb-1 text-xs text-muted-foreground">
		Add-only — labels can't be removed here, and apply to every copy of each image. Resaves are
		skipped.
	</p>
{/snippet}

{#snippet menuRow(
	label: string,
	Icon: typeof FolderPlus,
	onclick: () => void,
	opts?: { disabled?: boolean; deeper?: boolean; danger?: boolean }
)}
	<button
		type="button"
		{onclick}
		disabled={opts?.disabled}
		class="flex w-full items-center gap-3 rounded-lg px-3 py-3 text-left text-sm transition-colors hover:bg-muted disabled:pointer-events-none disabled:opacity-40 {opts?.danger
			? 'text-muted-foreground'
			: ''}"
	>
		<Icon class="size-4 shrink-0" />
		<span class="flex-1">{label}</span>
		{#if opts?.deeper}
			<ChevronRight class="size-4 shrink-0 text-muted-foreground" />
		{/if}
	</button>
{/snippet}

{#if sidebar.isMobile}
	<!-- Mobile: a floating pill instead of a bar, so the grid keeps its height. The
	     count rides on the pill and everything else lives in the drawer below. -->
	<div
		class="fixed left-1/2 z-40 -translate-x-1/2"
		style="bottom: calc(env(safe-area-inset-bottom) + 1.25rem)"
	>
		<Button size="lg" class="rounded-full shadow-lg" onclick={() => (mobileView = 'menu')}>
			<ListChecks class="size-4" />
			Bulk actions ({saves.length})
		</Button>
	</div>
{:else}
	<!-- Its own panel, stacked under the rounded main panel and inside the inset's
	     gutter, so it inherits the same margins and lines up with it. `slide` animates
	     the height; the panel above is flex-1, so it shrinks in step.
	     `|global` is required: transitions are local by default, and the block this
	     lives in isn't what changes — the page's `{#if selectMode}` around the whole
	     component is. A local transition would simply never play. -->
	<div
		transition:slide|global={{ duration: 200, easing: cubicOut }}
		class="mt-2 shrink-0 rounded-2xl bg-popover/95 shadow-sm backdrop-blur-sm"
	>
		<div class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-2 p-3">
			<div class="flex items-center gap-3 text-sm">
				<span class="font-medium">{saves.length} selected</span>
				<button
					type="button"
					class="text-xs text-muted-foreground underline-offset-2 hover:underline disabled:opacity-50"
					onclick={onSelectAll}
					disabled={saves.length >= selectableCount}
				>
					Select all loaded
				</button>
				{#if saves.length > 0}
					<button
						type="button"
						class="text-xs text-muted-foreground underline-offset-2 hover:underline"
						onclick={onClear}
					>
						Clear
					</button>
				{/if}
			</div>

			<div class="flex items-center gap-1.5">
				<Popover.Root bind:open={copyOpen}>
					<Popover.Trigger>
						{#snippet child({ props })}
							<Button {...props} variant="secondary" size="sm" disabled={saves.length === 0}>
								<FolderPlus class="size-4" />
								Copy
							</Button>
						{/snippet}
					</Popover.Trigger>
					<Popover.Content align="end" side="top" class="w-72 overflow-hidden p-0">
						{@render destinationList((uri) => {
							onCopy(uri);
							copyOpen = false;
						})}
					</Popover.Content>
				</Popover.Root>

				{#if canMove}
					<Popover.Root bind:open={moveOpen}>
						<Popover.Trigger>
							{#snippet child({ props })}
								<Button {...props} variant="secondary" size="sm" disabled={saves.length === 0}>
									<FolderInput class="size-4" />
									Move
								</Button>
							{/snippet}
						</Popover.Trigger>
						<Popover.Content align="end" side="top" class="w-72 overflow-hidden p-0">
							{@render destinationList((uri) => {
								onMove(uri);
								moveOpen = false;
							})}
						</Popover.Content>
					</Popover.Root>
				{/if}

				<Button variant="secondary" size="sm" disabled={saves.length === 0} onclick={downloadAll}>
					<Download class="size-4" />
					Download
				</Button>

				{#if ownContext}
					<Button
						variant="secondary"
						size="sm"
						disabled={blobCids.length === 0}
						onclick={() => (attributionOpen = true)}
					>
						<Quote class="size-4" />
						Attribution
					</Button>
					<Popover.Root bind:open={labelsOpen}>
						<Popover.Trigger>
							{#snippet child({ props })}
								<Button {...props} variant="secondary" size="sm" disabled={saves.length === 0}>
									<Tag class="size-4" />
									Labels
								</Button>
							{/snippet}
						</Popover.Trigger>
						<Popover.Content align="end" side="top" class="w-72">
							{@render labelBody()}
							<Button
								size="sm"
								class="mt-1 w-full"
								disabled={applying || chosen.size === 0}
								onclick={applyLabels}
							>
								{applying ? 'Applying…' : `Apply to ${saves.length}`}
							</Button>
						</Popover.Content>
					</Popover.Root>
				{/if}

				<Button variant="ghost" size="sm" onclick={onExit}>
					<X class="size-4" />
					Cancel
				</Button>
			</div>
		</div>
	</div>
{/if}

<!-- One mobile drawer, four views. Copy/Move/Labels swap the content in place
     rather than stacking a second drawer on the menu. -->
{#if sidebar.isMobile}
	<!-- Must be a two-way binding, not `open={...}` + onOpenChange. When the drawer
	     dismisses itself, vaul writes its own open state (use-drawer-root's
	     closeDrawer), and the tap-outside path gets there via onDialogOpenChange,
	     which calls closeDrawer(true) and so skips onOpenChange entirely. A one-way
	     prop never hears about it: the sheet is gone but mobileView still says
	     'menu', so re-tapping the pill assigns the same value and reopens nothing.
	     The setter is what pulls our state back. -->
	<Drawer.Root
		bind:open={() => mobileView !== null, (o) => (mobileView = o ? (mobileView ?? 'menu') : null)}
	>
		<Drawer.Content>
			{#if mobileView === 'copy' || mobileView === 'move'}
				<Drawer.Header>
					<Drawer.Title>
						{mobileView === 'move' ? 'Move to collection' : 'Copy to collection'}
					</Drawer.Title>
					<Drawer.Description>
						Pick a destination for the {saves.length} selected
						{saves.length === 1 ? 'image' : 'images'}.
					</Drawer.Description>
				</Drawer.Header>
				{@render destinationList(pick)}
				<Drawer.Footer>
					<Button variant="outline" onclick={() => (mobileView = 'menu')}>
						<ChevronLeft class="size-4" />
						Back
					</Button>
				</Drawer.Footer>
			{:else if mobileView === 'labels'}
				<Drawer.Header>
					<Drawer.Title>Add labels</Drawer.Title>
					<Drawer.Description>Flag sensitive or AI-generated content.</Drawer.Description>
				</Drawer.Header>
				<div class="p-3">
					{@render labelBody()}
				</div>
				<Drawer.Footer>
					<Button disabled={applying || chosen.size === 0} onclick={applyLabels}>
						{applying ? 'Applying…' : `Apply to ${saves.length}`}
					</Button>
					<Button variant="outline" onclick={() => (mobileView = 'menu')}>
						<ChevronLeft class="size-4" />
						Back
					</Button>
				</Drawer.Footer>
			{:else}
				<Drawer.Header>
					<Drawer.Title>Bulk actions</Drawer.Title>
					<Drawer.Description>
						{saves.length}
						{saves.length === 1 ? 'image' : 'images'} selected.
					</Drawer.Description>
				</Drawer.Header>

				<div class="flex items-center gap-3 px-4 pb-1 text-xs">
					<button
						type="button"
						class="text-muted-foreground underline-offset-2 disabled:opacity-40"
						onclick={onSelectAll}
						disabled={saves.length >= selectableCount}
					>
						Select all loaded
					</button>
					{#if saves.length > 0}
						<button
							type="button"
							class="text-muted-foreground underline-offset-2"
							onclick={onClear}
						>
							Clear
						</button>
					{/if}
				</div>

				<div class="px-2 pb-2">
					{@render menuRow('Copy to collection', FolderPlus, () => (mobileView = 'copy'), {
						disabled: saves.length === 0,
						deeper: true
					})}
					{#if canMove}
						{@render menuRow('Move to collection', FolderInput, () => (mobileView = 'move'), {
							disabled: saves.length === 0,
							deeper: true
						})}
					{/if}
					{@render menuRow(
						'Download',
						Download,
						() => {
							mobileView = null;
							void downloadAll();
						},
						{ disabled: saves.length === 0 }
					)}
					{#if ownContext}
						{@render menuRow('Attribution', Quote, openAttributionFromMenu, {
							disabled: blobCids.length === 0
						})}
						{@render menuRow('Labels', Tag, () => (mobileView = 'labels'), {
							disabled: saves.length === 0,
							deeper: true
						})}
					{/if}
					{@render menuRow(
						'Cancel selection',
						X,
						() => {
							mobileView = null;
							onExit();
						},
						{ danger: true }
					)}
				</div>
			{/if}
		</Drawer.Content>
	</Drawer.Root>
{/if}

<BulkAttributionDialog bind:open={attributionOpen} {blobCids} onDone={onExit} />
