<script lang="ts">
	// Native share target. An OS share carries its own intent — "put this in Currents" —
	// so this page asks the one question left (which collection) and gets out of the way,
	// instead of dropping the user on /upload, which is a workbench for a different job.
	// Details (alt text, attribution, labels) are filled later in organize mode.
	//
	// Two shapes arrive here. An image is one decision: pick a collection, save, leave.
	// A link is a page scrape that can yield several images, so it needs a pick step
	// first — a *selection*, not a delete-what-you-didn't-want list, since the user never
	// asked for those images individually.
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetch } from '$lib/api';
	import { uploadBlobDirect, DirectUploadError } from '$lib/blob-upload';
	import { isNative } from '$lib/platform';
	import { share, type PendingShare } from '$lib/stores/share.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import Check from '@lucide/svelte/icons/check';
	import X from '@lucide/svelte/icons/x';

	type Phase = 'fetching' | 'select' | 'saving' | 'done' | 'error';

	type Item = {
		id: string;
		previewUrl: string;
		file?: File;
		imageUrl?: string;
		pageUrl?: string;
		selected: boolean;
		status: 'pending' | 'saving' | 'done' | 'error';
	};

	let phase = $state<Phase>('fetching');
	let items = $state<Item[]>([]);
	let pickerOpen = $state(false);
	let savedUri = $state('');
	let errorText = $state('');
	// A single shared image never needs the pick step, and gets to leave on its own.
	let singleImage = $state(false);

	let objectUrls: string[] = [];

	let selected = $derived(items.filter((i) => i.selected));
	let savedCount = $derived(items.filter((i) => i.status === 'done').length);
	let failedCount = $derived(items.filter((i) => i.status === 'error').length);
	let savedName = $derived(
		savedUri === ''
			? 'your profile'
			: (collections.items.find((c) => c.uri === savedUri)?.name ?? 'your collection')
	);
	let organizeHref = $derived(
		savedUri ? `${resolve('/organize')}?c=${encodeURIComponent(savedUri)}` : resolve('/organize')
	);

	let pending = $state<PendingShare | null>(null);
	let started = false;

	onMount(() => {
		// The route only exists inside the native shell; the web share target still
		// routes to /upload (see share-target.ts).
		if (!isNative()) {
			goto(resolve('/'));
			return;
		}
		pending = untrack(() => share.pending);
		if (!pending) goto(resolve('/'));

		return () => {
			for (const u of objectUrls) URL.revokeObjectURL(u);
		};
	});

	// The share deliberately stays in the store until we know there's a session to act
	// on. A share can arrive logged out, or with a session the appview no longer accepts,
	// and the layout bounces those to the welcome screen — leaving it in the store means
	// logging in brings the user back here with the image intact instead of dropping it.
	// Consumed only once we're past that, so a stale share can't resurface later.
	$effect(() => {
		if (!pending || started || !auth.checked || !auth.user) return;
		started = true;
		const share_ = pending;
		share.pending = null;
		if (share_.type === 'image') {
			items = share_.files.map((file, i) => {
				const url = URL.createObjectURL(file);
				objectUrls.push(url);
				// Shared images were chosen deliberately in the other app's picker, so they
				// all start selected — deselecting is the correction, unlike a page scrape
				// where nothing was asked for individually.
				return { id: `shared-${i}`, previewUrl: url, file, selected: true, status: 'pending' };
			});
			singleImage = items.length === 1;
			phase = 'select';
			if (singleImage) pickerOpen = true;
		} else {
			void loadFromUrl(share_.url);
		}
	});

	// Hand the share back to the store and let the session be re-established; the login
	// forms route here again once it is.
	function bounceToLogin() {
		auth.user = null;
		if (pending) share.pending = pending;
		goto(resolve('/'));
	}

	async function loadFromUrl(pageUrl: string) {
		phase = 'fetching';
		try {
			const res = await apiFetch('/api/extract-images', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ url: pageUrl })
			});
			if (res.status === 401) {
				bounceToLogin();
				return;
			}
			if (!res.ok) {
				fail('Could not read images from that link.');
				return;
			}
			const found: string[] = (await res.json()).images ?? [];
			if (found.length === 0) {
				fail('No images found at that link.');
				return;
			}
			items = found.map((imageUrl, i) => ({
				id: `url-${i}`,
				previewUrl: imageUrl,
				imageUrl,
				pageUrl,
				// A page with one image is as unambiguous as a shared image, so preselect
				// it; with several, the user picks and nothing is chosen for them.
				selected: found.length === 1,
				status: 'pending'
			}));
			phase = 'select';
			if (found.length === 1) pickerOpen = true;
		} catch {
			fail('Network error while reading that link.');
		}
	}

	function fail(message: string) {
		errorText = message;
		phase = 'error';
	}

	function toggle(id: string) {
		const item = items.find((i) => i.id === id);
		if (item) item.selected = !item.selected;
	}

	// A preview that won't load is a dead scrape result — drop it rather than let the
	// user select something that can't be saved.
	function dropBroken(id: string) {
		items = items.filter((i) => i.id !== id);
	}

	// Returning to the app the user shared from is the natural end of the flow — but
	// only after any confirmation has been readable, and never on failure, where
	// leaving would lose the image silently.
	async function leave() {
		try {
			const { SendIntent } = await import('send-intent');
			SendIntent.finish();
		} catch {
			goto(resolve('/'));
		}
	}

	async function saveOne(item: Item, collectionUri: string): Promise<'ok' | 'stop'> {
		item.status = 'saving';
		try {
			const form = new FormData();
			if (item.file) {
				// Upload straight to the user's PDS (own IP -> own rate-limit bucket). No
				// server-side fallback: a dead session (401) re-auths; a missing rpc: scope
				// (403) already fired the reconnect prompt in uploadBlobDirect and stops the
				// batch so the user reconnects rather than silently draining the shared bucket.
				try {
					form.append('blob', JSON.stringify(await uploadBlobDirect(item.file)));
				} catch (e) {
					item.status = 'error';
					if (e instanceof DirectUploadError && e.phase === 'token' && e.status === 401) {
						if (savedCount === 0) {
							bounceToLogin();
							return 'stop';
						}
						auth.user = null;
						errorText = 'Your session expired before the rest could be saved.';
						return 'stop';
					}
					if (e instanceof DirectUploadError && e.status === 403) {
						errorText = 'Reconnect your account to upload.';
						return 'stop';
					}
					return 'ok';
				}
			} else if (item.imageUrl) {
				form.append('imageUrl', item.imageUrl);
				if (item.pageUrl) form.append('url', item.pageUrl);
			}
			form.append('collection', collectionUri);
			const res = await apiFetch('/save', {
				method: 'POST',
				body: form,
				headers: { Accept: 'application/json' }
			});
			if (!res.ok) {
				item.status = 'error';
				// A dead session or a rate-limited PDS will fail every remaining item too,
				// so stop rather than burn through the batch.
				if (res.status === 401) {
					// Only safe to hand the batch back to the login round trip while nothing
					// has been written yet — otherwise re-running it would duplicate the saves
					// that did land, so stop and let the user retry deliberately.
					if (savedCount === 0) {
						bounceToLogin();
						return 'stop';
					}
					auth.user = null;
					errorText = 'Your session expired before the rest could be saved.';
					return 'stop';
				}
				if (res.status === 429) {
					errorText = 'Your PDS is rate limiting uploads. Try again in a minute.';
					return 'stop';
				}
				return 'ok';
			}
			item.status = 'done';
		} catch {
			item.status = 'error';
			errorText = 'Network error. Some images were not saved.';
			return 'stop';
		}
		return 'ok';
	}

	async function saveTo(uri: string) {
		const batch = selected;
		if (batch.length === 0) return;
		savedUri = uri;
		errorText = '';
		phase = 'saving';
		// Sequential on purpose: these are PDS writes against a per-user budget, and a
		// share batch is small enough that latency isn't worth a rate-limit risk.
		for (const item of batch) {
			if ((await saveOne(item, uri)) === 'stop') break;
		}
		phase = 'done';
		// One image, everything saved, nothing to explain — hand the user straight back,
		// but not before the confirmation has been on screen long enough to read.
		if (singleImage && failedCount === 0) setTimeout(leave, 1600);
	}

	function retry() {
		errorText = '';
		for (const item of items) if (item.status === 'error') item.status = 'pending';
		phase = 'select';
		pickerOpen = true;
	}
</script>

<svelte:head><title>Save to Currents</title></svelte:head>

<div class="flex h-dvh flex-col pt-[env(safe-area-inset-top)] pb-[env(safe-area-inset-bottom)]">
	<header class="flex shrink-0 items-center justify-between gap-2 px-4 py-3">
		<h1 class="text-base font-semibold">
			{phase === 'done' ? 'Saved to Currents' : 'Save to Currents'}
		</h1>
		<Button variant="ghost" size="icon-sm" aria-label="Close" onclick={leave}>
			<X class="size-4" />
		</Button>
	</header>

	{#if phase === 'fetching'}
		<div class="flex flex-1 flex-col items-center justify-center gap-3 px-6">
			<Spinner class="size-6 text-muted-foreground" />
			<p class="text-sm text-muted-foreground">Looking for images…</p>
		</div>
	{:else if phase === 'done' && !singleImage}
		<!-- Terminal state as page content, not a dialog stacked on top: the drawer has
		     closed and there is nothing left to decide. -->
		<div class="flex flex-1 flex-col items-center justify-center gap-4 px-6 text-center">
			<div class="flex size-12 items-center justify-center rounded-full bg-secondary">
				<Check class="size-6 text-green-600" />
			</div>
			<div class="space-y-1">
				<p class="font-medium">
					{savedCount}
					{savedCount === 1 ? 'image' : 'images'} saved to {savedName}
				</p>
				{#if failedCount > 0}
					<p class="text-sm text-destructive">
						{failedCount} couldn't be saved{errorText ? ` — ${errorText}` : '.'}
					</p>
				{/if}
				<p class="text-sm text-muted-foreground">
					Add alt text, attribution and labels any time in organize mode.
				</p>
			</div>
			<div class="flex w-full max-w-xs flex-col gap-2">
				<Button href={organizeHref} class="w-full">Add details in organize</Button>
				<Button variant="outline" class="w-full" onclick={leave}>Done</Button>
			</div>
		</div>
	{:else if items.length === 1}
		<div class="flex min-h-0 flex-1 items-center justify-center px-4">
			<img
				src={items[0].previewUrl}
				alt=""
				class="max-h-full max-w-full rounded-2xl object-contain shadow-sm"
			/>
		</div>
	{:else}
		<div class="min-h-0 flex-1 overflow-y-auto px-4">
			<p class="pb-3 text-sm text-muted-foreground">
				{selected.length > 0
					? `${selected.length} of ${items.length} selected`
					: 'Tap the images you want to save.'}
			</p>
			<div class="grid grid-cols-3 gap-2 pb-4">
				{#each items as item (item.id)}
					<button
						type="button"
						class="relative aspect-square overflow-hidden rounded-xl border-2 transition-colors {item.selected
							? 'border-primary'
							: 'border-transparent'}"
						aria-pressed={item.selected}
						disabled={phase !== 'select'}
						onclick={() => toggle(item.id)}
					>
						<img
							src={item.previewUrl}
							alt=""
							class="size-full object-cover {item.selected ? '' : 'opacity-60'}"
							onerror={() => dropBroken(item.id)}
						/>
						{#if item.status === 'saving'}
							<span class="absolute inset-0 grid place-items-center bg-background/60">
								<Spinner class="size-5" />
							</span>
						{:else if item.status === 'done'}
							<span class="absolute inset-0 grid place-items-center bg-background/60">
								<Check class="size-5 text-green-600" />
							</span>
						{:else if item.status === 'error'}
							<span
								class="text-destructive-foreground absolute inset-x-0 bottom-0 bg-destructive/90 py-0.5 text-[10px]"
							>
								Failed
							</span>
						{:else if item.selected}
							<span
								class="absolute top-1.5 right-1.5 grid size-5 place-items-center rounded-full bg-primary"
							>
								<Check class="size-3 text-primary-foreground" />
							</span>
						{/if}
					</button>
				{/each}
			</div>
		</div>
	{/if}

	<!-- The multi-image 'done' state is the full-page screen above; a single image keeps
	     its preview and gets its confirmation down here, in the space the Save button
	     occupied, so the layout doesn't jump before the app hands back. -->
	{#if phase !== 'fetching' && !(phase === 'done' && !singleImage)}
		<div class="flex shrink-0 flex-col gap-3 px-4 pt-4 pb-6">
			{#if phase === 'done'}
				{#if failedCount === 0}
					<!-- h-10 matches the size="lg" Save button it replaces, so nothing shifts. -->
					<p class="flex h-10 items-center justify-center gap-2 text-sm font-medium">
						<Check class="size-4 text-green-600" />
						Saved to {savedName}
					</p>
				{:else}
					<p class="text-center text-sm text-destructive">
						{errorText || "That image couldn't be saved."}
					</p>
					<Button class="w-full" onclick={retry}>Try again</Button>
				{/if}
			{:else if phase === 'saving'}
				<p class="flex items-center justify-center gap-2 text-sm text-muted-foreground">
					<Spinner class="size-4" />
					Saving {savedCount + 1} of {selected.length}…
				</p>
			{:else if phase === 'error'}
				<p class="text-center text-sm text-destructive">{errorText}</p>
				{#if items.length > 0}
					<Button class="w-full" onclick={retry}>Try again</Button>
				{:else}
					<Button variant="outline" class="w-full" onclick={leave}>Close</Button>
				{/if}
			{:else}
				<CollectionSelector variant="drawer" bind:open={pickerOpen} onSelect={saveTo}>
					{#snippet trigger({ props })}
						<Button
							{...props}
							size="lg"
							class="w-full rounded-full"
							disabled={selected.length === 0}
						>
							{selected.length > 1 ? `Save ${selected.length} images` : 'Save to collection'}
						</Button>
					{/snippet}
				</CollectionSelector>
			{/if}
		</div>
	{/if}
</div>
