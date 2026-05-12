<script lang="ts">
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { getImageContent, type SaveAttribution, type SaveView } from '$lib/types';

	interface Props {
		open: boolean;
		save: SaveView;
		onSaved: (attribution: SaveAttribution) => void;
	}

	let { open = $bindable(), save, onSaved }: Props = $props();

	let credit = $state('');
	let license = $state('');
	let url = $state('');
	let submitting = $state(false);
	let error = $state<string | null>(null);

	let resavePending = $derived(
		save.viewer?.saves?.some((s) => s.saveUri === 'optimistic') ?? false
	);

	$effect(() => {
		if (!open) return;
		untrack(() => {
			const current = save.viewer?.attribution;
			credit = current?.credit ?? '';
			license = current?.license ?? '';
			url = current?.url ?? '';
			error = null;
		});
	});

	async function submit(e: Event) {
		e.preventDefault();
		const blobCid = getImageContent(save)?.blobCid;
		if (!blobCid) {
			error = 'This save has no image content to attribute.';
			return;
		}
		submitting = true;
		error = null;
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/save/attribution`, {
				method: 'PUT',
				credentials: 'include',
				headers: {
					'Content-Type': 'application/x-www-form-urlencoded',
					Accept: 'application/json'
				},
				body: new URLSearchParams({
					blob_cid: blobCid,
					attribution_url: url.trim(),
					attribution_license: license.trim(),
					attribution_credit: credit.trim()
				}).toString()
			});
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
					open = false;
					return;
				}
				error = `Failed to update (${res.status}).`;
				return;
			}
			onSaved({
				credit: credit.trim() || undefined,
				license: license.trim() || undefined,
				url: url.trim() || undefined
			});
			toast.success('Attribution updated');
			open = false;
		} catch {
			error = 'Network error. Please try again.';
		} finally {
			submitting = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Attribution</Dialog.Title>
			<Dialog.Description>
				Credit the source of this image. Your attribution applies to every collection of yours that
				contains it.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={submit} class="space-y-4">
			<div class="space-y-2">
				<Label for="save-attribution-credit">Credit</Label>
				<Input
					id="save-attribution-credit"
					bind:value={credit}
					maxlength={500}
					disabled={submitting}
					placeholder="e.g. Jane Doe"
				/>
			</div>
			<div class="space-y-2">
				<Label for="save-attribution-license">License</Label>
				<Input
					id="save-attribution-license"
					bind:value={license}
					maxlength={200}
					disabled={submitting}
					placeholder="e.g. CC BY 4.0"
				/>
			</div>
			<div class="space-y-2">
				<Label for="save-attribution-url">Attribution URL</Label>
				<Input
					id="save-attribution-url"
					type="url"
					bind:value={url}
					maxlength={2000}
					disabled={submitting}
					placeholder="https://example.com/source"
				/>
			</div>
			{#if error}
				<p class="text-sm text-destructive">{error}</p>
			{/if}
			<Dialog.Footer>
				<Button
					type="button"
					variant="outline"
					onclick={() => (open = false)}
					disabled={submitting}
				>
					Cancel
				</Button>
				<Button type="submit" disabled={submitting || resavePending}>
					{submitting ? 'Saving…' : resavePending ? 'Waiting for save…' : 'Save'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
