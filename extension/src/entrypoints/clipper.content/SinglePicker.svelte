<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { clipper, defaultCollectionUri, hideClipper } from '../../lib/clipper-store.svelte';
	import CollectionField from '../../lib/CollectionField.svelte';
	import SaveDetails, { newDetails } from '../../lib/SaveDetails.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';

	interface Props {
		onPickerOpenChange?: (open: boolean) => void;
	}
	let { onPickerOpenChange }: Props = $props();

	type SaveState = 'idle' | 'saving' | 'saved' | 'error';
	let saveState = $state<SaveState>('idle');
	let errorMsg = $state('');
	// The user's explicit pick (null until they choose); otherwise we fall back to
	// the most-recently-used default, which moves as collections load in.
	let picked = $state<string | null>(null);
	let creatingCollection = $state(false);
	let collectionUri = $derived(picked ?? defaultCollectionUri(clipper.collections));
	let alt = $state('');
	const details = $state(newDetails(clipper.siteHints.attributionCredit ?? ''));
	const labels = new SvelteSet<string>();

	// Pre-fill the alt field if this exact image already has alt text in the
	// network. Best-effort; never overwrites text the user has already typed.
	async function suggestAlt(imgUrl: string) {
		try {
			const response = await browser.runtime.sendMessage({ type: 'LOOKUP_ALT', imgUrl });
			if (response?.alt && clipper.imgUrl === imgUrl && !alt.trim()) {
				alt = response.alt;
			}
		} catch {
			// best-effort suggestion
		}
	}
	void suggestAlt(clipper.imgUrl);

	async function save() {
		saveState = 'saving';
		try {
			const response = await browser.runtime.sendMessage({
				type: 'SAVE_IMAGE',
				imgUrl: clipper.imgUrl,
				collectionUri,
				text: details.note.trim(),
				alt: alt.trim(),
				originUrl: clipper.originUrl,
				attributionUrl: details.attributionUrl.trim(),
				attributionLicense: details.attributionLicense.trim(),
				attributionCredit: details.attributionCredit.trim(),
				labels: Array.from(labels).join(',')
			});
			if (response.ok) {
				saveState = 'saved';
				setTimeout(hideClipper, 1500);
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
		}
	}

	let busy = $derived(saveState === 'saving' || saveState === 'saved');
</script>

<img
	class="max-h-[20vh] w-full shrink-0 rounded-2xl bg-muted object-contain"
	src={clipper.imgUrl}
	alt="Preview"
/>

<div class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto">
	<CollectionField
		selectedUri={collectionUri}
		bind:picked
		bind:creating={creatingCollection}
		disabled={busy}
		onOpenChange={onPickerOpenChange}
	/>

	<div class="flex flex-col gap-1">
		<span class="text-xs text-muted-foreground">Alt text</span>
		<Textarea
			placeholder="Describe the image (optional but recommended)"
			bind:value={alt}
			disabled={busy}
			maxlength={2000}
			rows={2}
		/>
	</div>

	<SaveDetails {details} {labels} disabled={busy} />
</div>

<div class="flex shrink-0 flex-col gap-2">
	{#if saveState === 'saved'}
		<p class="text-center font-medium">Saved!</p>
	{:else}
		<Button onclick={save} disabled={creatingCollection || saveState === 'saving'}>
			{saveState === 'saving' ? 'Saving…' : 'Save to Currents'}
		</Button>
		{#if saveState === 'error'}
			<p class="text-xs text-destructive">{errorMsg}</p>
		{/if}
	{/if}
</div>
