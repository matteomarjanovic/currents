<script lang="ts">
	import { toast } from 'svelte-sonner';
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

	// Copy/Move: two popovers on desktop (each anchored to its button); a shared
	// drawer on mobile.
	let copyOpen = $state(false);
	let moveOpen = $state(false);
	let mobilePicker = $state<'copy' | 'move' | null>(null);
	function pick(dest: string) {
		if (mobilePicker === 'move') onMove(dest);
		else if (mobilePicker === 'copy') onCopy(dest);
		mobilePicker = null;
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

<div class="shrink-0 border-t bg-popover/95 p-3 backdrop-blur-sm">
	<div class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-2">
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
			<!-- Copy -->
			{#if sidebar.isMobile}
				<Button
					variant="secondary"
					size="sm"
					disabled={saves.length === 0}
					onclick={() => (mobilePicker = 'copy')}
				>
					<FolderPlus class="size-4" />
					Copy
				</Button>
			{:else}
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
			{/if}

			<!-- Move -->
			{#if canMove}
				{#if sidebar.isMobile}
					<Button
						variant="secondary"
						size="sm"
						disabled={saves.length === 0}
						onclick={() => (mobilePicker = 'move')}
					>
						<FolderInput class="size-4" />
						Move
					</Button>
				{:else}
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
				{#if sidebar.isMobile}
					<Button
						variant="secondary"
						size="sm"
						disabled={saves.length === 0}
						onclick={() => (labelsOpen = true)}
					>
						<Tag class="size-4" />
						Labels
					</Button>
				{:else}
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
			{/if}

			<Button variant="ghost" size="sm" onclick={onExit}>
				<X class="size-4" />
				Cancel
			</Button>
		</div>
	</div>
</div>

<!-- Mobile copy/move destination drawer (desktop uses the popovers above). -->
{#if sidebar.isMobile}
	<Drawer.Root open={mobilePicker !== null} onOpenChange={(o) => !o && (mobilePicker = null)}>
		<Drawer.Content>
			<Drawer.Header>
				<Drawer.Title>
					{mobilePicker === 'move' ? 'Move to collection' : 'Copy to collection'}
				</Drawer.Title>
				<Drawer.Description>
					Pick a destination for the {saves.length} selected
					{saves.length === 1 ? 'image' : 'images'}.
				</Drawer.Description>
			</Drawer.Header>
			{@render destinationList(pick)}
			<Drawer.Footer>
				<Drawer.Close>
					{#snippet child({ props })}
						<Button {...props} variant="outline">Cancel</Button>
					{/snippet}
				</Drawer.Close>
			</Drawer.Footer>
		</Drawer.Content>
	</Drawer.Root>

	<!-- Mobile labels drawer. -->
	<Drawer.Root bind:open={labelsOpen}>
		<Drawer.Content>
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
				<Drawer.Close>
					{#snippet child({ props })}
						<Button {...props} variant="outline">Cancel</Button>
					{/snippet}
				</Drawer.Close>
			</Drawer.Footer>
		</Drawer.Content>
	</Drawer.Root>
{/if}

<BulkAttributionDialog bind:open={attributionOpen} {blobCids} onDone={onExit} />
