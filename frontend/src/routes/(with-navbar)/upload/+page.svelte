<script lang="ts">
	import { apiFetch } from '$lib/api';
	import { onDestroy } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Progress } from '$lib/components/ui/progress';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Popover from '$lib/components/ui/popover';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import Camera from '@lucide/svelte/icons/camera';
	import Images from '@lucide/svelte/icons/images';
	import X from '@lucide/svelte/icons/x';
	import Check from '@lucide/svelte/icons/check';
	import TriangleAlert from '@lucide/svelte/icons/alert-triangle';
	import { toast } from 'svelte-sonner';
	import { RATE_LIMIT_MESSAGE } from '$lib/rate-limit';
	import { blobCidFromBytes } from '$lib/blob-cid';
	import { isNative } from '$lib/platform';
	import { share } from '$lib/stores/share.svelte';

	type StagedStatus = 'pending' | 'uploading' | 'done' | 'error';
	type Staged = {
		id: string;
		url: string; // preview src: object URL for local files, remote URL for pasted images
		status: StagedStatus;
		error?: string;
		alt?: string;
		altSuggested?: boolean;
		file?: File; // present for local uploads
		imageUrl?: string; // paste-from-URL: the appview downloads this server-side
		pageUrl?: string; // source page, saved as the record's originUrl
	};

	let staged = $state<Staged[]>([]);
	// undefined = nothing chosen yet; '' = the profile (unsorted, no collection).
	let selectedCollectionUri = $state<string | undefined>(undefined);
	let selectedSelfLabels = $state<Set<string>>(new Set());
	let uploading = $state(false);
	let completed = $state(false);
	let rateLimited = $state(false);
	let popoverDismissed = $state(false);
	let dragActive = $state(false);
	let dragDepth = 0;
	let fileInputEl: HTMLInputElement | undefined = $state();
	let sourceUrl = $state('');
	let fetching = $state(false);

	const SELF_LABEL_OPTIONS: { val: string; label: string }[] = [
		{ val: 'porn', label: 'Porn' },
		{ val: 'sexual', label: 'Sexual' },
		{ val: 'nudity', label: 'Nudity' },
		{ val: 'graphic-media', label: 'Graphic' },
		{ val: 'currents-ai-generated', label: 'AI-generated' }
	];

	function toggleSelfLabel(val: string) {
		const next = new Set(selectedSelfLabels);
		if (next.has(val)) next.delete(val);
		else next.add(val);
		selectedSelfLabels = next;
	}

	let total = $derived(staged.length);
	let doneCount = $derived(staged.filter((s) => s.status === 'done').length);
	let errorCount = $derived(staged.filter((s) => s.status === 'error').length);
	let processed = $derived(doneCount + errorCount);
	let progressValue = $derived(total === 0 ? 0 : (processed / total) * 100);
	let canSave = $derived(!uploading && total > 0 && selectedCollectionUri !== undefined);
	let popoverOpen = $derived((uploading || completed || rateLimited) && !popoverDismissed);

	function addFiles(files: FileList | File[]) {
		const imgs = Array.from(files).filter((f) => f.type.startsWith('image/'));
		const mapped: Staged[] = imgs.map((file) => ({
			id: `${file.name}-${file.size}-${file.lastModified}-${Math.random()}`,
			file,
			url: URL.createObjectURL(file),
			status: 'pending'
		}));
		staged = [...staged, ...mapped];
		for (const item of mapped) if (item.file) void prefillAlt(item.id, item.file);
	}

	const native = isNative();

	// On Android the web <input type="file"> hands back a content:// File the WebView can't
	// preview or upload, so native uses the Camera plugin's picker, which returns a webPath we
	// fetch into a real File.
	async function pickFromNative(source: 'camera' | 'gallery') {
		const {
			Camera: NativeCamera,
			CameraResultType,
			CameraSource
		} = await import('@capacitor/camera');
		try {
			const photo = await NativeCamera.getPhoto({
				resultType: CameraResultType.Uri,
				source: source === 'camera' ? CameraSource.Camera : CameraSource.Photos,
				quality: 90,
				correctOrientation: true,
				allowEditing: false
			});
			if (!photo.webPath) return;
			const blob = await (await fetch(photo.webPath)).blob();
			const ext = (photo.format ?? 'jpeg').toLowerCase();
			const file = new File([blob], `capture-${Date.now()}.${ext}`, {
				type: blob.type || `image/${ext}`,
				lastModified: Date.now()
			});
			addFiles([file]);
		} catch (err) {
			// user cancelled or plugin error
			console.warn('native picker', err);
		}
	}

	// If this exact image already has alt text somewhere in the network, pre-fill
	// the field with it as a suggestion (best-effort; never overwrites typed text).
	async function prefillAlt(id: string, file: File) {
		try {
			const cid = await blobCidFromBytes(await file.arrayBuffer());
			const res = await apiFetch(`/api/blob/alt?cid=${encodeURIComponent(cid)}`);
			if (!res.ok) return;
			const data: { alt?: string } = await res.json();
			const item = staged.find((s) => s.id === id);
			if (item && data.alt && !item.alt?.trim()) {
				item.alt = data.alt;
				item.altSuggested = true;
			}
		} catch {
			// Best-effort; ignore (e.g. CID mismatch after an oversized image is downscaled).
		}
	}

	// Paste-from-URL: ask the appview to scrape a page (it fetches server-side to
	// avoid CORS), then stage the images directly, as if uploaded from disk.
	async function fetchFromUrl() {
		const u = sourceUrl.trim();
		if (!u || fetching) return;
		// Client-side check is UX only — the appview re-validates the scheme and
		// blocks internal addresses, since a direct caller can bypass this.
		try {
			const proto = new URL(u).protocol;
			if (proto !== 'http:' && proto !== 'https:') throw new Error();
		} catch {
			toast.error('Enter a valid http(s) URL (including https://).');
			return;
		}
		fetching = true;
		try {
			const res = await apiFetch(`/api/extract-images`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ url: u })
			});
			if (res.status === 401) {
				auth.user = null;
				promptLogin();
				return;
			}
			if (!res.ok) {
				toast.error('Could not fetch images from that URL.');
				return;
			}
			const data: { images?: string[] } = await res.json();
			const imgs = data.images ?? [];
			if (imgs.length === 0) {
				toast.error('No images found at that URL.');
				return;
			}
			// Stage them directly. Unreachable previews auto-remove (the staged
			// grid's onerror), which also filters scraped junk like dead links.
			const mapped: Staged[] = imgs.map((imageUrl) => ({
				id: `url-${imageUrl}-${Math.random()}`,
				url: imageUrl,
				imageUrl,
				pageUrl: u,
				status: 'pending'
			}));
			staged = [...staged, ...mapped];
			sourceUrl = '';
			toast.success(`Added ${mapped.length} image${mapped.length === 1 ? '' : 's'} from the page.`);
		} catch {
			toast.error('Could not fetch images from that URL.');
		} finally {
			fetching = false;
		}
	}

	function removeStaged(id: string) {
		const item = staged.find((s) => s.id === id);
		if (item?.file) URL.revokeObjectURL(item.url);
		staged = staged.filter((s) => s.id !== id);
	}

	// Copy each pick into an in-memory File before resetting the input. On Android Chrome a File
	// from a file input is backed by a content:// reference that's released the moment the input's
	// selection is cleared, so the lazy reads behind the preview's object URL (and the upload)
	// raced input.value='' and randomly came back broken. Read sequentially while the input still
	// holds the selection; in-memory copies are stable.
	async function onFilePick(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const picked = input.files
			? Array.from(input.files).filter((f) => f.type.startsWith('image/'))
			: [];
		for (const f of picked) {
			try {
				const copy = new File([await f.arrayBuffer()], f.name, {
					type: f.type,
					lastModified: f.lastModified
				});
				addFiles([copy]);
			} catch {
				// Unreadable pick (e.g. a cloud file that never finished downloading) — skip it.
			}
		}
		input.value = '';
	}

	// Consume an image/link shared to the app from the OS share sheet (src/lib/share-target.ts).
	$effect(() => {
		const p = share.pending;
		if (!p) return;
		share.pending = null;
		if (p.type === 'image') {
			addFiles([p.file]);
		} else {
			sourceUrl = p.url;
			void fetchFromUrl();
		}
	});

	function onDragEnter(e: DragEvent) {
		if (!e.dataTransfer?.types.includes('Files')) return;
		dragDepth++;
		dragActive = true;
	}
	function onDragOver(e: DragEvent) {
		if (!e.dataTransfer?.types.includes('Files')) return;
		e.preventDefault();
	}
	function onDragLeave() {
		dragDepth = Math.max(0, dragDepth - 1);
		if (dragDepth === 0) dragActive = false;
	}
	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragDepth = 0;
		dragActive = false;
		if (e.dataTransfer?.files.length) addFiles(e.dataTransfer.files);
	}

	async function uploadOne(item: Staged): Promise<'ok' | 'unauthorized' | 'rate-limited'> {
		item.status = 'uploading';
		try {
			const form = new FormData();
			if (item.file) {
				form.append('image', item.file, item.file.name);
			} else if (item.imageUrl) {
				form.append('imageUrl', item.imageUrl);
				if (item.pageUrl) form.append('url', item.pageUrl);
			}
			form.append('collection', selectedCollectionUri ?? '');
			if (item.alt?.trim()) {
				form.append('alt', item.alt.trim());
			}
			if (selectedSelfLabels.size > 0) {
				form.append('labels', Array.from(selectedSelfLabels).join(','));
			}
			const res = await apiFetch(`/save`, {
				method: 'POST',
				body: form,
				headers: { Accept: 'application/json' }
			});
			if (!res.ok) {
				if (res.status === 401) {
					item.status = 'pending';
					return 'unauthorized';
				}
				// Rate-limited by the PDS: leave the item pending so a retry picks it
				// back up, and let the caller stop launching new uploads.
				if (res.status === 429) {
					item.status = 'pending';
					return 'rate-limited';
				}
				item.status = 'error';
				item.error = (await res.text()).trim() || `HTTP ${res.status}`;
				return 'ok';
			}
			item.status = 'done';
		} catch (e) {
			item.status = 'error';
			item.error = String(e);
		}
		return 'ok';
	}

	async function startUpload() {
		if (!canSave) return;
		uploading = true;
		completed = false;
		rateLimited = false;
		popoverDismissed = false;

		const queue = staged.filter((s) => s.status !== 'done');
		let cursor = 0;
		let unauthorized = false;
		let hitRateLimit = false;

		async function worker() {
			while (!unauthorized && !hitRateLimit) {
				const i = cursor++;
				if (i >= queue.length) return;
				const result = await uploadOne(queue[i]);
				if (result === 'unauthorized') unauthorized = true;
				else if (result === 'rate-limited') hitRateLimit = true;
			}
		}

		const concurrency = Math.min(5, queue.length);
		await Promise.all(Array.from({ length: concurrency }, worker));

		uploading = false;
		if (unauthorized) {
			auth.user = null;
			promptLogin();
			return;
		}
		// Stop on the first rate-limit instead of turning the whole batch red:
		// remaining items stay pending so "Retry remaining" can resume them.
		if (hitRateLimit) {
			rateLimited = true;
			toast.error(RATE_LIMIT_MESSAGE);
			return;
		}
		completed = true;
	}

	onDestroy(() => {
		for (const s of staged) if (s.file) URL.revokeObjectURL(s.url);
	});
</script>

<svelte:head>
	<title>Upload · Currents</title>
</svelte:head>

<svelte:window
	ondragenter={onDragEnter}
	ondragover={onDragOver}
	ondragleave={onDragLeave}
	ondrop={onDrop}
/>

<div class="mx-auto w-full max-w-3xl space-y-6 pb-24">
	<h1 class="text-2xl font-semibold">Upload your images</h1>
	<p class="text-sm text-muted-foreground">
		Select the images you want to upload, choose a collection to save them in, and start the upload.
	</p>
	<p class="text-sm text-muted-foreground">IMPORTANT: Collections and saves are public for now.</p>

	<div class="flex justify-between space-y-2">
		<!-- <div class="text-sm font-medium">Select the collection where to save your images</div> -->
		<div class="hidden md:block">
			<CollectionSelector
				variant="popover"
				selectedUri={selectedCollectionUri}
				onSelect={(uri) => (selectedCollectionUri = uri)}
			/>
		</div>
		<div class="md:hidden">
			<CollectionSelector
				variant="drawer"
				selectedUri={selectedCollectionUri}
				onSelect={(uri) => (selectedCollectionUri = uri)}
			/>
		</div>
		<div class="flex justify-end">
			<Button onclick={startUpload} disabled={!canSave}>
				{uploading ? 'Uploading…' : 'Start upload'}
			</Button>
		</div>
	</div>

	<div class="flex flex-wrap items-center gap-2 text-xs">
		<span class="text-muted-foreground">Apply labels (to all images in this batch):</span>
		{#each SELF_LABEL_OPTIONS as opt (opt.val)}
			{@const active = selectedSelfLabels.has(opt.val)}
			<button
				type="button"
				onclick={() => toggleSelfLabel(opt.val)}
				disabled={uploading}
				class="rounded-full border px-2.5 py-1 transition-colors {active
					? 'border-foreground bg-foreground text-background'
					: 'border-border text-muted-foreground hover:bg-muted'}"
			>
				{opt.label}
			</button>
		{/each}
	</div>

	<div class="flex flex-wrap items-center gap-3">
		{#if native}
			<Button variant="secondary" onclick={() => pickFromNative('gallery')}>
				<Images class="size-4" />
				Choose from gallery
			</Button>
			<Button variant="secondary" onclick={() => pickFromNative('camera')}>
				<Camera class="size-4" />
				Take photo
			</Button>
		{:else}
			<Button variant="secondary" onclick={() => fileInputEl?.click()}>
				<ImagePlus class="size-4" />
				Add files
			</Button>
			{#if total === 0}
				<span> ... or drag and drop them in the page </span>
			{/if}
		{/if}
		<input
			bind:this={fileInputEl}
			type="file"
			accept="image/*"
			multiple
			class="hidden"
			onchange={onFilePick}
		/>
		{#if total > 0}
			<span class="text-sm text-muted-foreground">
				{total} image{total === 1 ? '' : 's'} staged
			</span>
		{/if}
	</div>

	<div class="flex items-center gap-2">
		<Input
			type="url"
			placeholder="…or paste a page or image URL"
			bind:value={sourceUrl}
			disabled={fetching || uploading}
			onkeydown={(e) => {
				if (e.key === 'Enter') {
					e.preventDefault();
					fetchFromUrl();
				}
			}}
		/>
		<Button
			variant="secondary"
			onclick={fetchFromUrl}
			disabled={!sourceUrl.trim() || fetching || uploading}
		>
			{fetching ? 'Fetching…' : 'Fetch'}
		</Button>
	</div>

	{#if staged.length > 0}
		<div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
			{#each staged as item (item.id)}
				<div class="group relative aspect-square overflow-hidden rounded-xl bg-muted">
					<!-- lazy + async decode so staging many full-res phone photos doesn't decode
					     them all at once: mobile browsers cap image memory and drop decodes
					     (random "broken" thumbnails) when the whole grid renders eagerly. -->
					<img
						src={item.url}
						alt=""
						loading="lazy"
						decoding="async"
						class="size-full object-cover"
						onerror={() => {
							if (item.imageUrl) removeStaged(item.id);
						}}
					/>
					{#if item.status === 'uploading'}
						<div
							class="absolute inset-0 flex items-center justify-center bg-black/40 text-xs font-medium text-white"
						>
							Uploading…
						</div>
					{:else if item.status === 'done'}
						<div class="absolute inset-0 flex items-center justify-center bg-black/40 text-white">
							<Check class="size-8" />
						</div>
					{:else if item.status === 'error'}
						<div
							class="absolute inset-0 flex items-center justify-center bg-destructive/70 px-2 text-center text-xs text-white"
							title={item.error}
						>
							Upload failed
						</div>
					{/if}
					{#if !uploading && item.status !== 'done'}
						<Popover.Root>
							<Popover.Trigger>
								{#snippet child({ props })}
									<button
										{...props}
										type="button"
										class="absolute top-1.5 left-1.5 flex h-7 items-center gap-1 rounded-full px-2 text-[11px] font-semibold transition-colors hover:opacity-90 {item.alt?.trim()
											? 'bg-primary text-primary-foreground'
											: 'bg-black/60 text-white'}"
										aria-label="Add alt text"
									>
										ALT
										{#if item.alt?.trim()}<Check class="size-3" />{/if}
									</button>
								{/snippet}
							</Popover.Trigger>
							<Popover.Content align="start" class="w-80 gap-0 space-y-2">
								<div class="flex justify-center overflow-hidden rounded-md bg-muted">
									<img src={item.url} alt="" class="max-h-48 w-auto object-contain" />
								</div>
								<Label for={`alt-${item.id}`} class="pt-2 text-xs font-medium"
									>Alt text{#if item.altSuggested}<span class="font-normal text-muted-foreground">
											· suggested, edit if needed</span
										>{/if}</Label
								>
								<Textarea
									id={`alt-${item.id}`}
									bind:value={item.alt}
									oninput={() => (item.altSuggested = false)}
									maxlength={2000}
									rows={3}
									placeholder="Describe this image for people who use screen readers."
								/>
							</Popover.Content>
						</Popover.Root>
						<button
							type="button"
							class="absolute top-1.5 right-1.5 flex size-7 items-center justify-center rounded-full bg-black/60 text-white transition-colors hover:bg-black/80"
							onclick={() => removeStaged(item.id)}
							aria-label="Remove image"
						>
							<X class="size-4" />
						</button>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if dragActive}
	<div
		class="pointer-events-none fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm"
	>
		<div
			class="rounded-2xl border-2 border-dashed border-foreground/40 px-8 py-6 text-lg font-medium"
		>
			Drop images to upload
		</div>
	</div>
{/if}

{#if popoverOpen}
	<div
		class="fixed bottom-6 left-1/2 z-40 w-[min(92vw,26rem)] -translate-x-1/2 rounded-2xl border bg-popover/95 p-4 shadow-lg backdrop-blur-sm"
	>
		{#if uploading}
			<div class="space-y-3">
				<div>
					<div class="font-medium">Uploading images</div>
					<div class="text-xs text-muted-foreground">
						Don't close this page to avoid interrupting the upload.
					</div>
				</div>
				<Progress value={progressValue} />
				<div class="text-sm text-muted-foreground">{processed} of {total}</div>
			</div>
		{:else if completed}
			<div class="space-y-3">
				<div class="flex items-center gap-2">
					<Check class="size-5 text-primary" />
					<div class="font-medium">Upload complete</div>
					<button
						type="button"
						class="ml-auto rounded-full p-1 text-muted-foreground hover:bg-foreground/10"
						onclick={() => (popoverDismissed = true)}
						aria-label="Dismiss"
					>
						<X class="size-4" />
					</button>
				</div>
				<div class="text-sm text-muted-foreground">
					{doneCount} uploaded{errorCount > 0 ? ` · ${errorCount} failed` : ''}
				</div>
				{#if selectedCollectionUri}
					<Button
						variant="default"
						class="w-full"
						href={`/profile/${auth.user?.handle}/collection/${selectedCollectionUri.split('/').pop()}`}
					>
						Go to collection
					</Button>
				{:else}
					<Button variant="default" class="w-full" href={`/profile/${auth.user?.handle}`}>
						Go to profile
					</Button>
				{/if}
			</div>
		{:else if rateLimited}
			<div class="space-y-3">
				<div class="flex items-center gap-2">
					<TriangleAlert class="size-5 text-destructive" />
					<div class="font-medium">Upload paused</div>
					<button
						type="button"
						class="ml-auto rounded-full p-1 text-muted-foreground hover:bg-foreground/10"
						onclick={() => (popoverDismissed = true)}
						aria-label="Dismiss"
					>
						<X class="size-4" />
					</button>
				</div>
				<div class="text-sm text-muted-foreground">{RATE_LIMIT_MESSAGE}</div>
				{#if doneCount > 0}
					<div class="text-xs text-muted-foreground">{doneCount} uploaded so far.</div>
				{/if}
				<Button variant="default" class="w-full" onclick={startUpload}>Retry remaining</Button>
			</div>
		{/if}
	</div>
{/if}
