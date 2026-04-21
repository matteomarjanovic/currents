<script lang="ts">
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { onDestroy } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Progress } from '$lib/components/ui/progress';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import X from '@lucide/svelte/icons/x';
	import Check from '@lucide/svelte/icons/check';

	type StagedStatus = 'pending' | 'uploading' | 'done' | 'error';
	type Staged = {
		id: string;
		file: File;
		url: string;
		status: StagedStatus;
		error?: string;
	};

	let staged = $state<Staged[]>([]);
	let selectedCollectionUri = $state<string>('');
	let uploading = $state(false);
	let completed = $state(false);
	let popoverDismissed = $state(false);
	let dragActive = $state(false);
	let dragDepth = 0;
	let fileInputEl: HTMLInputElement | undefined = $state();

	let total = $derived(staged.length);
	let doneCount = $derived(staged.filter((s) => s.status === 'done').length);
	let errorCount = $derived(staged.filter((s) => s.status === 'error').length);
	let processed = $derived(doneCount + errorCount);
	let progressValue = $derived(total === 0 ? 0 : (processed / total) * 100);
	let canSave = $derived(!uploading && total > 0 && !!selectedCollectionUri);
	let popoverOpen = $derived((uploading || completed) && !popoverDismissed);

	function addFiles(files: FileList | File[]) {
		const imgs = Array.from(files).filter((f) => f.type.startsWith('image/'));
		const mapped: Staged[] = imgs.map((file) => ({
			id: `${file.name}-${file.size}-${file.lastModified}-${Math.random()}`,
			file,
			url: URL.createObjectURL(file),
			status: 'pending'
		}));
		staged = [...staged, ...mapped];
	}

	function removeStaged(id: string) {
		const item = staged.find((s) => s.id === id);
		if (item) URL.revokeObjectURL(item.url);
		staged = staged.filter((s) => s.id !== id);
	}

	function onFilePick(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		if (input.files) addFiles(input.files);
		input.value = '';
	}

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

	async function startUpload() {
		if (!canSave) return;
		uploading = true;
		completed = false;
		popoverDismissed = false;
		for (const item of staged) {
			if (item.status === 'done') continue;
			item.status = 'uploading';
			try {
				const form = new FormData();
				form.append('image', item.file, item.file.name);
				form.append('collection', selectedCollectionUri);
				const res = await fetch(`${PUBLIC_APPVIEW_URL}/save`, {
					method: 'POST',
					body: form,
					credentials: 'include',
					headers: { Accept: 'application/json' }
				});
				if (!res.ok) {
					if (res.status === 401) {
						auth.user = null;
						promptLogin();
						item.status = 'pending';
						uploading = false;
						return;
					}
					item.status = 'error';
					item.error = (await res.text()).trim() || `HTTP ${res.status}`;
					continue;
				}
				item.status = 'done';
			} catch (e) {
				item.status = 'error';
				item.error = String(e);
			}
		}
		uploading = false;
		completed = true;
	}

	onDestroy(() => {
		for (const s of staged) URL.revokeObjectURL(s.url);
	});
</script>

<svelte:window
	ondragenter={onDragEnter}
	ondragover={onDragOver}
	ondragleave={onDragLeave}
	ondrop={onDrop}
/>

<div class="mx-auto w-full max-w-3xl space-y-6 pb-24">
	<h1 class="text-2xl font-semibold">Upload your images</h1>

	<div class="space-y-2">
		<div class="text-sm font-medium">Collection</div>
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
	</div>

	<div class="flex items-center gap-3">
		<Button variant="secondary" onclick={() => fileInputEl?.click()}>
			<ImagePlus class="size-4" />
			Add files
		</Button>
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

	{#if staged.length > 0}
		<div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
			{#each staged as item (item.id)}
				<div class="group relative aspect-square overflow-hidden rounded-xl bg-muted">
					<img src={item.url} alt="" class="size-full object-cover" />
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
						<button
							type="button"
							class="absolute top-1.5 right-1.5 flex size-7 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition-opacity group-hover:opacity-100 hover:bg-black/80 focus-visible:opacity-100"
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

	<div class="flex justify-end">
		<Button onclick={startUpload} disabled={!canSave}>
			{uploading ? 'Uploading…' : 'Save'}
		</Button>
	</div>
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
						href={`/collection/${encodeURIComponent(selectedCollectionUri)}`}
					>
						Go to collection
					</Button>
				{/if}
			</div>
		{/if}
	</div>
{/if}
