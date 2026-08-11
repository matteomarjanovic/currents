<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { clipper, defaultCollectionUri, hideClipper } from '../../lib/clipper-store.svelte';
	import CollectionField from '../../lib/CollectionField.svelte';
	import SaveDetails, { newDetails } from '../../lib/SaveDetails.svelte';
	import { Button } from '$lib/components/ui/button';
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';

	interface Props {
		onPickerOpenChange?: (open: boolean) => void;
	}
	let { onPickerOpenChange }: Props = $props();

	// Three at a time. Each save is an uploadBlob plus a createRecord against the
	// user's own PDS, which bills both against an hourly write budget, so the run
	// stays deliberately unhurried rather than firing everything at once.
	const CONCURRENCY = 3;

	type ItemState = 'saving' | 'saved' | 'error';

	// A snapshot: the page can mutate under us while the panel is open.
	const candidates = clipper.candidates;

	// Everything starts selected: the discovery filter is what makes that a sane
	// default, and deselecting a few is less work than picking most of them.
	const selected = new SvelteSet(candidates.map((c) => c.url));
	// The user's explicit pick (null until they choose); otherwise the
	// most-recently-used default, which moves as collections load in.
	let picked = $state<string | null>(null);
	let creatingCollection = $state(false);
	let collectionUri = $derived(picked ?? defaultCollectionUri(clipper.collections));
	// Note, labels and attribution are stamped onto every image in the run — one
	// page is usually one source, one photographer, one licence.
	const details = $state(newDetails());
	const labels = new SvelteSet<string>();
	let showDetails = $state(false);
	let states = $state<Record<string, ItemState>>({});
	let running = $state(false);
	let finished = $state(false);
	// Set when a run gave up early (rate limit) — the remaining images are
	// untouched and the user can start them again.
	let stoppedMsg = $state('');
	let runTotal = $state(0);
	let runDone = $state(0);

	let locked = $derived(running || finished);
	// Shown on the collapsed row so filled-in fields aren't invisible.
	let detailCount = $derived(
		[
			details.note,
			details.attributionCredit,
			details.attributionUrl,
			details.attributionLicense
		].filter((v) => v.trim()).length + labels.size
	);
	let savedCount = $derived(Object.values(states).filter((s) => s === 'saved').length);
	let failedCount = $derived(Object.values(states).filter((s) => s === 'error').length);

	function toggle(url: string) {
		if (locked) return;
		if (!selected.delete(url)) selected.add(url);
	}

	// Skips what already landed, so "All" after a run can't duplicate saves.
	function selectAll() {
		for (const c of candidates) if (states[c.url] !== 'saved') selected.add(c.url);
	}

	async function run(urls: string[]) {
		running = true;
		finished = false;
		stoppedMsg = '';
		runTotal = urls.length;
		runDone = 0;
		clipper.locked = true;

		const queue = [...urls];
		let stopped = false;

		async function worker() {
			for (;;) {
				const url = stopped ? undefined : queue.shift();
				if (!url) return;
				const candidate = candidates.find((c) => c.url === url);
				if (!candidate) continue;
				states[url] = 'saving';
				try {
					const res = await browser.runtime.sendMessage({
						type: 'SAVE_IMAGE',
						imgUrl: candidate.url,
						collectionUri,
						// The page's own alt attribute — the one piece of metadata a bulk
						// save gets for free that the single-image dialog has to ask for.
						alt: candidate.alt,
						originUrl: clipper.originUrl,
						text: details.note.trim(),
						attributionUrl: details.attributionUrl.trim(),
						attributionLicense: details.attributionLicense.trim(),
						attributionCredit: details.attributionCredit.trim(),
						labels: Array.from(labels).join(',')
					});
					states[url] = res.ok ? 'saved' : 'error';
					if (res.authError) {
						stopped = true;
						clipper.authState = 'unauthenticated';
						clipper.reauthNeeded = res.reauth ?? false;
					} else if (res.rateLimited) {
						// Grinding through the rest would just collect more 429s.
						stopped = true;
						stoppedMsg = res.error ?? '';
					}
				} catch (e) {
					states[url] = 'error';
					stoppedMsg ||= String(e);
				}
				runDone += 1;
			}
		}

		// A worker only marks an image once it has claimed it, so whatever an abort
		// left in the queue stays unmarked and selected — "Save" picks it up again.
		await Promise.all(Array.from({ length: Math.min(CONCURRENCY, queue.length) }, worker));

		running = false;
		finished = true;
		clipper.locked = false;
	}

	function start() {
		const urls = candidates.filter((c) => selected.has(c.url)).map((c) => c.url);
		if (urls.length) void run(urls);
	}

	function retryFailed() {
		const urls = candidates.filter((c) => states[c.url] === 'error').map((c) => c.url);
		if (urls.length) void run(urls);
	}

	// Back to picking, with the images that just landed deselected.
	function resume() {
		finished = false;
		for (const url of Object.keys(states)) {
			if (states[url] === 'saved') selected.delete(url);
		}
	}
</script>

<div class="flex shrink-0 items-center gap-2 pr-8">
	<span class="text-sm font-medium">
		{candidates.length}
		{candidates.length === 1 ? 'image' : 'images'} on this page
	</span>
	{#if candidates.length && !locked}
		<div class="ml-auto flex gap-1">
			<Button variant="ghost" size="xs" onclick={selectAll}>All</Button>
			<Button variant="ghost" size="xs" onclick={() => selected.clear()}>None</Button>
		</div>
	{/if}
</div>

{#if candidates.length === 0}
	<p class="py-6 text-center text-sm text-muted-foreground">
		No images large enough to save were found on this page.
	</p>
{:else}
	<div
		class="-mx-1 scrollbar-hide grid min-h-0 flex-1 grid-cols-4 gap-1.5 overflow-y-auto px-1 py-2 sm:grid-cols-5 md:grid-cols-6"
	>
		{#each candidates as c (c.url)}
			{@const state = states[c.url]}
			{@const isSelected = selected.has(c.url)}
			<button
				type="button"
				onclick={() => toggle(c.url)}
				disabled={locked}
				aria-pressed={isSelected}
				aria-label={c.alt || 'Image'}
				class="group relative aspect-square h-fit overflow-hidden rounded-xl bg-muted ring-2 transition {isSelected
					? 'ring-foreground'
					: 'opacity-40 ring-transparent grayscale hover:opacity-70'}"
			>
				<img src={c.url} alt="" loading="lazy" class="size-full object-cover" />

				{#if state === 'saving'}
					<span class="absolute inset-0 flex items-center justify-center bg-background/70">
						<LoaderCircle class="size-5 animate-spin" />
					</span>
				{:else if state === 'saved'}
					<span class="absolute inset-0 flex items-center justify-center bg-background/70">
						<Check class="size-5" />
					</span>
				{:else if state === 'error'}
					<span class="absolute inset-0 flex items-center justify-center bg-destructive/25">
						<TriangleAlert class="size-5 text-destructive" />
					</span>
				{:else}
					{#if isSelected}
						<span
							class="absolute top-1.5 right-1.5 flex size-5 items-center justify-center rounded-full bg-foreground text-background"
						>
							<Check class="size-3" />
						</span>
					{/if}
					<!-- The cheapest signal for "this one isn't worth saving". -->
					<span
						class="absolute bottom-1.5 left-1.5 rounded-md bg-black/60 px-1 py-px text-[10px] leading-tight text-white opacity-0 transition-opacity group-hover:opacity-100"
					>
						{c.width}×{c.height}
					</span>
				{/if}
			</button>
		{/each}
	</div>

	<div class="flex shrink-0 flex-col gap-2">
		{#if running}
			<div class="h-1.5 overflow-hidden rounded-full bg-muted">
				<div
					class="h-full rounded-full bg-foreground transition-[width]"
					style="width:{runTotal ? (runDone / runTotal) * 100 : 0}%"
				></div>
			</div>
			<p class="text-center text-xs text-muted-foreground">Saving {runDone} of {runTotal}…</p>
		{:else if finished}
			<p class="text-center text-sm font-medium">
				Saved {savedCount}{failedCount ? ` · ${failedCount} failed` : ''}
			</p>
			{#if stoppedMsg}
				<p class="text-xs text-destructive">{stoppedMsg}</p>
			{/if}
			<div class="flex gap-2">
				{#if failedCount}
					<Button variant="outline" class="flex-1" onclick={retryFailed}>Retry failed</Button>
				{:else}
					<Button variant="outline" class="flex-1" onclick={resume}>Pick more</Button>
				{/if}
				<Button class="flex-1" onclick={hideClipper}>Done</Button>
			</div>
		{:else}
			<CollectionField
				selectedUri={collectionUri}
				bind:picked
				bind:creating={creatingCollection}
				onOpenChange={onPickerOpenChange}
			/>

			<!-- Collapsed by default: the grid is what the panel is for, and these
			     fields would otherwise squeeze it out of the dialog. -->
			<Button
				variant="ghost"
				size="sm"
				class="justify-start self-start px-1.5 text-muted-foreground"
				onclick={() => (showDetails = !showDetails)}
			>
				{#if showDetails}
					<ChevronDown />
				{:else}
					<ChevronRight />
				{/if}
				Details for every image
				{#if detailCount && !showDetails}
					<span class="text-foreground">({detailCount})</span>
				{/if}
			</Button>
			{#if showDetails}
				<div class="scrollbar-hide flex max-h-52 flex-col gap-3 overflow-y-auto">
					<SaveDetails
						{details}
						{labels}
						notePlaceholder="Add the same note to every image (optional)"
						labelsLabel="Apply labels to every image:"
					/>
				</div>
			{/if}

			<Button onclick={start} disabled={selected.size === 0 || creatingCollection}>
				Save {selected.size}
				{selected.size === 1 ? 'image' : 'images'}
			</Button>
		{/if}
	</div>
{/if}
