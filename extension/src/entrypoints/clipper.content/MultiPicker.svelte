<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { clipper, hideClipper, rememberCollection } from '../../lib/clipper-store.svelte';
	import { collectPageImages } from './collect-images';
	import MultiCollectionSelector from '../../lib/MultiCollectionSelector.svelte';
	import SaveDetails, { newDetails, type Details } from '../../lib/SaveDetails.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';
	import { scrollFade } from '../../lib/scroll-fade';
	import Check from '@lucide/svelte/icons/check';
	import ChevronsDown from '@lucide/svelte/icons/chevrons-down';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import Plus from '@lucide/svelte/icons/plus';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';

	// Each image needs one upload and one record per collection. Three images at a
	// time keeps the user's PDS write budget from being exhausted in one burst.
	const CONCURRENCY = 3;
	type ItemState = 'saving' | 'saved' | 'error';
	type Step = 'images' | 'details' | 'collections';
	type ImageDetails = { alt: string; details: Details; labels: SvelteSet<string> };
	let step = $state<Step>('images');
	let candidates = $state([...clipper.candidates]);
	const selected = new SvelteSet<string>();
	const destinations = new SvelteSet<string>();
	let imageDetails = $state<Record<string, ImageDetails>>({});
	let editingUrl = $state('');
	let creatingCollection = $state(false);
	let states = $state<Record<string, ItemState>>({});
	let savedByUrl = $state<Record<string, string[]>>({});
	let running = $state(false);
	let finished = $state(false);
	let stoppedMsg = $state('');
	let runTotal = $state(0);
	let runDone = $state(0);
	let canScrollPage = $state(false);
	let preview = $derived(candidates.find((c) => c.url === editingUrl));
	let selectedUrls = $derived(candidates.filter((c) => selected.has(c.url)).map((c) => c.url));
	let savedCount = $derived(
		selectedUrls.filter((url) => [...destinations].every((uri) => savedByUrl[url]?.includes(uri)))
			.length
	);
	let remainingCount = $derived(selectedUrls.length - savedCount);
	let hasPending = $derived(
		selectedUrls.some((url) => [...destinations].some((uri) => !savedByUrl[url]?.includes(uri)))
	);

	function scanPage() {
		const fresh = collectPageImages();
		const known = new Map(candidates.map((c) => [c.url, c]));
		const added = fresh.filter((c) => {
			const current = known.get(c.url);
			if (!current) return true;
			if (current.width !== c.width) current.width = c.width;
			if (current.height !== c.height) current.height = c.height;
			if (current.alt !== c.alt) current.alt = c.alt;
			return false;
		});
		if (added.length) candidates = [...candidates, ...added].slice(0, 100);
		canScrollPage =
			(document.scrollingElement?.scrollHeight ?? document.documentElement.scrollHeight) >
				window.innerHeight + 24 || Array.from(document.images).some((img) => !img.complete);
	}

	// The panel leaves the page interactive. Scrolling can load lazy images and
	// infinite feeds; refresh discovery without changing the user's selection.
	$effect(() => {
		if (step !== 'images' || finished) return;
		let timer: ReturnType<typeof setTimeout> | undefined;
		const schedule = () => {
			clearTimeout(timer);
			timer = setTimeout(scanPage, 250);
		};
		const observer = new MutationObserver(schedule);
		observer.observe(document.documentElement, {
			childList: true,
			subtree: true,
			attributes: true,
			attributeFilter: ['src', 'srcset']
		});
		document.addEventListener('scroll', schedule, true);
		document.addEventListener('load', schedule, true);
		scanPage();
		return () => {
			clearTimeout(timer);
			observer.disconnect();
			document.removeEventListener('scroll', schedule, true);
			document.removeEventListener('load', schedule, true);
		};
	});

	function toggle(url: string) {
		if (!selected.delete(url)) selected.add(url);
	}

	function openDetails(url: string) {
		const candidate = candidates.find((c) => c.url === url);
		if (!candidate) return;
		imageDetails[url] ??= {
			alt: candidate.alt,
			details: newDetails(),
			labels: new SvelteSet<string>()
		};
		editingUrl = url;
		step = 'details';
	}

	function pendingUris(url: string, uris: string[]) {
		return uris.filter((uri) => !savedByUrl[url]?.includes(uri));
	}

	async function run() {
		const uris = [...destinations];
		const queue = selectedUrls.filter((url) => pendingUris(url, uris).length);
		if (!queue.length) return;
		running = true;
		finished = false;
		stoppedMsg = '';
		runTotal = queue.length;
		runDone = 0;
		clipper.locked = true;
		let stopped = false;
		const successfulDestinations = new Set<string>();

		async function worker() {
			for (;;) {
				const url = stopped ? undefined : queue.shift();
				if (!url) return;
				const candidate = candidates.find((c) => c.url === url);
				if (!candidate) continue;
				const metadata = imageDetails[url];
				states[url] = 'saving';
				try {
					const res = await browser.runtime.sendMessage({
						type: 'SAVE_IMAGE',
						imgUrl: url,
						collectionUris: pendingUris(url, uris),
						alt: metadata ? metadata.alt.trim() : candidate.alt,
						originUrl: clipper.originUrl,
						text: metadata?.details.note.trim() ?? '',
						attributionUrl: metadata?.details.attributionUrl.trim() ?? '',
						attributionLicense: metadata?.details.attributionLicense.trim() ?? '',
						attributionCredit: metadata?.details.attributionCredit.trim() ?? '',
						labels: metadata ? [...metadata.labels].join(',') : ''
					});
					if (res.savedCollections?.length) {
						for (const uri of res.savedCollections) successfulDestinations.add(uri);
						savedByUrl[url] = [...new Set([...(savedByUrl[url] ?? []), ...res.savedCollections])];
					}
					states[url] = pendingUris(url, uris).length ? 'error' : 'saved';
					if (res.authError) {
						stopped = true;
						clipper.authState = 'unauthenticated';
						clipper.reauthNeeded = res.reauth ?? false;
					} else if (res.rateLimited) {
						stopped = true;
						stoppedMsg = res.error ?? '';
					} else if (!res.ok) {
						stoppedMsg ||= res.error ?? 'Could not save an image';
					}
				} catch (e) {
					states[url] = 'error';
					stoppedMsg ||= String(e);
				}
				runDone += 1;
			}
		}

		await Promise.all(Array.from({ length: Math.min(CONCURRENCY, queue.length) }, worker));
		rememberCollection(uris.findLast((uri) => successfulDestinations.has(uri) && !!uri) ?? '');
		running = false;
		finished = true;
		clipper.locked = false;
	}

	function pickMore() {
		finished = false;
		step = 'images';
		for (const url of selectedUrls) if (states[url] === 'saved') selected.delete(url);
		states = {};
	}
</script>

{#if step === 'images'}
	<p class="shrink-0 font-medium">
		Select images <span class="text-muted-foreground">({selected.size} selected)</span>
	</p>
	{#if candidates.length === 0}
		<p class="py-8 text-center text-sm text-muted-foreground">
			No images large enough to save were found on this page.
		</p>
	{:else}
		<div
			use:scrollFade
			class="scrollbar-hide grid min-h-0 w-full flex-1 grid-cols-2 content-start gap-3 overflow-y-auto py-1"
		>
			{#each candidates as candidate (candidate.url)}
				{@const state = states[candidate.url]}
				{@const isSelected = selected.has(candidate.url)}
				<div
					class="group relative h-0 w-full min-w-0 overflow-hidden rounded-xl bg-transparent pb-[100%]"
				>
					<img
						src={candidate.url}
						alt=""
						loading="lazy"
						decoding="async"
						class="absolute inset-0 size-full object-cover transition-opacity {state === 'saved'
							? 'opacity-35'
							: isSelected
								? 'opacity-100'
								: 'opacity-60'}"
					/>
					<button
						type="button"
						onclick={() => toggle(candidate.url)}
						disabled={running || state === 'saved'}
						aria-pressed={isSelected}
						aria-label={candidate.alt || `Image, ${candidate.width} by ${candidate.height} pixels`}
						class="absolute inset-0 rounded-xl ring-2 transition-colors ring-inset {isSelected
							? 'ring-foreground'
							: 'ring-transparent hover:ring-border'}"
					></button>
					{#if isSelected && state !== 'saved'}
						<Button
							variant="secondary"
							size="xs"
							class="absolute top-2 left-2 z-10 rounded-lg bg-background/90 px-2 text-[11px] shadow-sm"
							onclick={() => openDetails(candidate.url)}
						>
							+ Add details
						</Button>
					{/if}
					{#if isSelected && state !== 'saved'}
						<span
							class="pointer-events-none absolute bottom-1.5 left-1.5 grid size-5 place-items-center rounded-full bg-foreground text-background"
							><Check class="size-3" /></span
						>
					{/if}
					<span
						class="pointer-events-none absolute right-1.5 bottom-1.5 rounded bg-black/65 px-1.5 py-0.5 text-[9px] leading-none font-medium text-white tabular-nums shadow-sm"
						>{candidate.width}×{candidate.height}</span
					>
					{#if state === 'saving'}
						<span class="absolute inset-0 grid place-items-center bg-background/65"
							><LoaderCircle class="size-5 animate-spin" /></span
						>
					{:else if state === 'saved'}
						<span class="absolute inset-0 grid place-items-center bg-background/65"
							><Check class="size-5" /></span
						>
					{:else if state === 'error'}
						<span
							class="absolute top-1.5 right-1.5 grid size-5 place-items-center rounded-full bg-destructive text-white"
							><TriangleAlert class="size-3" /></span
						>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
	<Button onclick={() => (step = 'collections')} disabled={!selected.size}>Next</Button>
	{#if canScrollPage && candidates.length < 100 && !finished}
		<p
			class="flex shrink-0 items-center justify-center gap-1 text-center text-xs text-muted-foreground"
		>
			<ChevronsDown class="size-3.5 shrink-0" />
			Scroll down the page to load more images
			<ChevronsDown class="size-3.5 shrink-0" />
		</p>
	{/if}
{:else if step === 'details'}
	<div class="flex shrink-0 items-center gap-2">
		<Button
			variant="outline"
			size="icon-sm"
			class="rounded-full"
			aria-label="Back"
			onclick={() => {
				editingUrl = '';
				step = 'images';
			}}><ChevronLeft class="size-4" /></Button
		>
		<span class="font-medium">Image details</span>
	</div>
	{#if preview && imageDetails[editingUrl]}
		<div class="shrink-0 bg-transparent">
			<img src={preview.url} alt="Preview" class="max-h-[24vh] w-full object-contain" />
		</div>
		<div use:scrollFade class="scrollbar-hide flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto">
			<div class="flex flex-col gap-1">
				<span class="text-xs text-muted-foreground">Alt text</span><Textarea
					placeholder="Describe the image (optional but recommended)"
					bind:value={imageDetails[editingUrl].alt}
					maxlength={2000}
					rows={2}
				/>
			</div>
			<SaveDetails
				details={imageDetails[editingUrl].details}
				labels={imageDetails[editingUrl].labels}
			/>
		</div>
	{/if}
	<Button
		onclick={() => {
			editingUrl = '';
			step = 'images';
		}}>Done</Button
	>
{:else}
	<div class="flex shrink-0 items-center justify-between gap-2">
		{#if !running && !finished && !creatingCollection}<Button
				variant="outline"
				size="icon-sm"
				class="rounded-full"
				aria-label="Back"
				onclick={() => (step = 'images')}><ChevronLeft class="size-4" /></Button
			>
			<Button
				variant="link"
				size="sm"
				class="h-auto p-0"
				onclick={() => (creatingCollection = true)}><Plus class="size-4" /> New collection</Button
			>
		{:else}
			<span class="font-medium">Choose collections</span>
		{/if}
	</div>
	<MultiCollectionSelector
		selected={destinations}
		bind:creating={creatingCollection}
		disabled={running || finished}
	/>
	{#if running}
		<div class="h-1.5 shrink-0 overflow-hidden rounded-full bg-muted">
			<div
				class="h-full bg-foreground transition-[width]"
				style="width:{runTotal ? (runDone / runTotal) * 100 : 0}%"
			></div>
		</div>
		<p class="text-center text-xs text-muted-foreground">Saving {runDone} of {runTotal} images…</p>
	{:else if finished}
		<p class="text-center text-sm font-medium">Saved {savedCount} of {selected.size} images</p>
		{#if stoppedMsg}<p class="text-xs text-destructive">{stoppedMsg}</p>{/if}
		<div class="flex gap-2">
			{#if remainingCount}<Button variant="outline" class="flex-1" onclick={run}
					>Retry remaining</Button
				>{:else}<Button variant="outline" class="flex-1" onclick={pickMore}>Pick more</Button>{/if}
			<Button class="flex-1" onclick={hideClipper}>Done</Button>
		</div>
	{:else}
		<Button
			onclick={run}
			disabled={!destinations.size ||
				!hasPending ||
				creatingCollection ||
				clipper.collectionsLoading}>Save</Button
		>
	{/if}
{/if}
