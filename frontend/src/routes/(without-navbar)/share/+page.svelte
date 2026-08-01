<script lang="ts">
	// Native share target. An OS share carries its own intent — "put this in Currents" —
	// so this page asks the one question left (which collection) and gets out of the way,
	// instead of dropping the user on /upload, which is a workbench for a different job.
	// Details (alt text, attribution, labels) are filled later in organize mode.
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetch } from '$lib/api';
	import { isNative } from '$lib/platform';
	import { share } from '$lib/stores/share.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections } from '$lib/stores/collections.svelte';
	import CollectionSelector from '$lib/components/collection-selector.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import Check from '@lucide/svelte/icons/check';
	import X from '@lucide/svelte/icons/x';

	type Phase = 'choosing' | 'saving' | 'saved' | 'error';

	let phase = $state<Phase>('choosing');
	let pickerOpen = $state(false);
	let file = $state<File | null>(null);
	let previewUrl = $state('');
	let savedUri = $state('');
	let errorText = $state('');

	let savedName = $derived(
		savedUri === ''
			? 'your profile'
			: (collections.items.find((c) => c.uri === savedUri)?.name ?? 'your collection')
	);

	onMount(() => {
		// The route only exists inside the native shell; the web share target still
		// routes to /upload (see share-target.ts).
		if (!isNative()) {
			goto(resolve('/'));
			return;
		}
		const pending = untrack(() => share.pending);
		if (pending?.type !== 'image') {
			goto(resolve('/'));
			return;
		}
		share.pending = null;
		file = pending.file;
		previewUrl = URL.createObjectURL(pending.file);
		// One image means the collection is the only decision left, so present the
		// drawer straight away rather than making the user tap a button first.
		pickerOpen = true;
		return () => URL.revokeObjectURL(previewUrl);
	});

	// Returning to the app the user shared from is the natural end of the flow — but
	// only after the confirmation has been on screen long enough to read, and never
	// on failure, where leaving would silently lose the image.
	async function leave() {
		try {
			const { SendIntent } = await import('send-intent');
			SendIntent.finish();
		} catch {
			goto(resolve('/'));
		}
	}

	function retry() {
		phase = 'choosing';
		pickerOpen = true;
	}

	async function saveTo(uri: string) {
		if (!file) return;
		phase = 'saving';
		errorText = '';
		try {
			const form = new FormData();
			form.append('image', file, file.name);
			form.append('collection', uri);
			const res = await apiFetch('/save', {
				method: 'POST',
				body: form,
				headers: { Accept: 'application/json' }
			});
			if (!res.ok) {
				if (res.status === 401) auth.user = null;
				errorText =
					res.status === 401
						? 'Your session expired. Open Currents and log in, then try again.'
						: res.status === 429
							? 'Your PDS is rate limiting uploads. Try again in a minute.'
							: (await res.text()).trim() || `Upload failed (${res.status}).`;
				phase = 'error';
				return;
			}
			savedUri = uri;
			phase = 'saved';
			setTimeout(leave, 1200);
		} catch {
			errorText = 'Network error. Your image has not been saved.';
			phase = 'error';
		}
	}
</script>

<svelte:head><title>Save to Currents</title></svelte:head>

<div class="flex h-dvh flex-col pt-[env(safe-area-inset-top)] pb-[env(safe-area-inset-bottom)]">
	<header class="flex items-center justify-between gap-2 px-4 py-3">
		<h1 class="text-base font-semibold">Save to Currents</h1>
		<Button variant="ghost" size="icon-sm" aria-label="Cancel" onclick={leave}>
			<X class="size-4" />
		</Button>
	</header>

	<div class="flex min-h-0 flex-1 items-center justify-center px-4">
		{#if previewUrl}
			<img
				src={previewUrl}
				alt=""
				class="max-h-full max-w-full rounded-2xl object-contain shadow-sm"
			/>
		{/if}
	</div>

	<div class="flex flex-col gap-3 px-4 pt-4 pb-6">
		{#if phase === 'saved'}
			<p class="flex items-center justify-center gap-2 text-sm font-medium">
				<Check class="size-4 text-green-600" />
				Saved to {savedName}
			</p>
		{:else if phase === 'saving'}
			<p class="flex items-center justify-center gap-2 text-sm text-muted-foreground">
				<Spinner class="size-4" />
				Saving…
			</p>
		{:else if phase === 'error'}
			<p class="text-center text-sm text-destructive">{errorText}</p>
			<Button class="w-full" onclick={retry}>Try again</Button>
		{:else}
			<CollectionSelector variant="drawer" bind:open={pickerOpen} onSelect={saveTo}>
				{#snippet trigger({ props })}
					<Button {...props} size="lg" class="w-full rounded-full">Save to collection</Button>
				{/snippet}
			</CollectionSelector>
		{/if}
	</div>
</div>
