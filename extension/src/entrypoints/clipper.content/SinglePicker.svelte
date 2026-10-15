<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { clipper, hideClipper, rememberCollection } from '../../lib/clipper-store.svelte';
	import MultiCollectionSelector from '../../lib/MultiCollectionSelector.svelte';
	import SaveDetails, { newDetails } from '../../lib/SaveDetails.svelte';
	import SaveLabels from '../../lib/SaveLabels.svelte';
	import SaveButton from '../../lib/SaveButton.svelte';
	import AltTextField from '../../lib/AltTextField.svelte';
	import { Button } from '$lib/components/ui/button';
	import { scrollFade } from '../../lib/scroll-fade';
	import { useViewNavigation } from '../../lib/view-navigation';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import Plus from '@lucide/svelte/icons/plus';

	type SaveState = 'idle' | 'saving' | 'saved' | 'error';
	let saveState = $state<SaveState>('idle');
	let errorMsg = $state('');
	const selected = new SvelteSet<string>();
	const saved = new SvelteSet<string>();
	let pendingUris = $derived([...selected].filter((uri) => !saved.has(uri)));
	let creatingCollection = $state(false);
	let editingDetails = $state(false);
	let width = $state(0);
	let height = $state(0);
	let alt = $state('');
	const details = $state(newDetails(clipper.siteHints.attributionCredit ?? ''));
	const labels = new SvelteSet<string>();
	const navigate = useViewNavigation();

	async function save() {
		const collectionUris = [...pendingUris];
		if (!collectionUris.length) return;
		saveState = 'saving';
		clipper.locked = true;
		try {
			const response = await browser.runtime.sendMessage({
				type: 'SAVE_IMAGE',
				imgUrl: clipper.imgUrl,
				collectionUris,
				text: details.note.trim(),
				alt: alt.trim(),
				originUrl: clipper.originUrl,
				attributionUrl: details.attributionUrl.trim(),
				attributionLicense: details.attributionLicense.trim(),
				attributionCredit: details.attributionCredit.trim(),
				labels: Array.from(labels).join(',')
			});
			for (const uri of response.savedCollections ?? []) saved.add(uri);
			rememberCollection(response.savedCollections?.findLast((uri: string) => !!uri) ?? '');
			if (response.ok) {
				saveState = 'saved';
				const session = clipper.session;
				setTimeout(() => {
					if (clipper.session === session) hideClipper();
				}, 1500);
			} else if (response.authError) {
				clipper.authState = 'unauthenticated';
				clipper.reauthNeeded = response.reauth ?? false;
			} else {
				saveState = 'error';
				errorMsg = response.error ?? 'Unknown error';
			}
		} catch (e) {
			saveState = 'error';
			errorMsg = String(e);
		} finally {
			clipper.locked = false;
		}
	}

	let busy = $derived(saveState === 'saving' || saveState === 'saved');
</script>

{#if editingDetails}
	<div class="flex shrink-0 items-center gap-2">
		<Button
			variant="outline"
			size="icon-sm"
			class="rounded-full"
			aria-label="Back"
			onclick={() => navigate(() => (editingDetails = false), -1)}
			><ChevronLeft class="size-4" /></Button
		>
		<span class="font-medium">Image details</span>
	</div>
{/if}

{#if !creatingCollection}
	<div class="flex shrink-0 justify-center">
		<div
			class="relative overflow-hidden rounded-xl bg-transparent {editingDetails
				? 'max-w-full'
				: 'aspect-square w-full max-w-[20vh]'}"
		>
			<img
				class={editingDetails
					? 'block max-h-[20vh] max-w-full object-contain'
					: 'absolute inset-0 size-full object-cover'}
				src={clipper.imgUrl}
				alt="Preview"
				bind:naturalWidth={width}
				bind:naturalHeight={height}
			/>
			{#if !editingDetails && !creatingCollection}
				<Button
					variant="secondary"
					size="xs"
					class="absolute right-2 bottom-2 z-10 rounded-lg bg-background/90 px-2 text-[11px] shadow-sm"
					disabled={busy}
					onclick={() => navigate(() => (editingDetails = true))}>+ Add details</Button
				>
				{#if !alt.trim()}
					<span
						class="pointer-events-none absolute top-6 left-1.5 rounded bg-black/65 px-1.5 py-0.5 text-[9px] leading-none text-white"
						>Alt missing</span
					>
				{/if}
			{/if}
			{#if width && height}
				<span
					class="pointer-events-none absolute top-1.5 left-1.5 rounded bg-black/65 px-1.5 py-0.5 text-[9px] leading-none font-medium text-white tabular-nums shadow-sm"
					>{width}×{height}</span
				>
			{/if}
		</div>
	</div>
{/if}

{#if editingDetails}
	<div use:scrollFade class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto">
		<AltTextField
			imageUrl={clipper.imgUrl}
			suggestion={clipper.altText}
			bind:value={alt}
			disabled={busy}
		/>

		<SaveDetails {details} {labels} showLabels={false} disabled={busy} />
	</div>
	<Button onclick={() => navigate(() => (editingDetails = false), -1)}>Done</Button>
{:else}
	{#if !creatingCollection}
		<SaveLabels {labels} disabled={busy} />
		<div class="flex shrink-0 items-center justify-between gap-2">
			<span class="font-medium">Save to</span>
			<Button
				variant="link"
				size="sm"
				class="h-auto p-0"
				disabled={busy || clipper.collectionsLoading}
				onclick={() => navigate(() => (creatingCollection = true))}
				><Plus class="size-4" /> New collection</Button
			>
		</div>
	{/if}
	<MultiCollectionSelector
		{selected}
		bind:creating={creatingCollection}
		disabled={busy || clipper.collectionsLoading}
	/>

	{#if !creatingCollection}
		<div class="flex shrink-0 flex-col gap-2">
			<SaveButton
				state={saveState === 'error' ? 'idle' : saveState}
				onclick={save}
				disabled={busy || !pendingUris.length || clipper.collectionsLoading}
			/>
			{#if saveState === 'error'}
				<p class="text-xs text-destructive">
					{#if saved.size}Saved to {saved.size}
						{saved.size === 1 ? 'destination' : 'destinations'}.{' '}
					{/if}{errorMsg}
				</p>
			{/if}
		</div>
	{/if}
{/if}
