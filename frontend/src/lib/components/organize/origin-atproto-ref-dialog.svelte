<script lang="ts">
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { apiFetch } from '$lib/api';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import type { SaveView } from '$lib/types';

	let {
		open = $bindable(),
		save,
		onSaved
	}: {
		open: boolean;
		save: SaveView;
		onSaved: (ref: SaveView['originAtprotoRef'] | null) => void;
	} = $props();

	let uri = $state('');
	let cid = $state('');
	let submitting = $state(false);
	let error = $state<string | null>(null);

	$effect(() => {
		if (!open) return;
		untrack(() => {
			uri = save.originAtprotoRef?.uri ?? '';
			cid = save.originAtprotoRef?.cid ?? '';
			error = null;
		});
	});

	async function submit(e: Event) {
		e.preventDefault();
		const nextUri = uri.trim();
		const nextCid = cid.trim();
		if (!!nextUri !== !!nextCid) {
			error = 'Enter both the record URI and CID, or leave both empty to remove the reference.';
			return;
		}
		const ref = nextUri ? { uri: nextUri, cid: nextCid } : null;
		const saveUri = save.uri;
		const rkey = saveUri.split('/').pop();
		submitting = true;
		error = null;
		try {
			const res = await apiFetch(`/api/save/${rkey}/origin-atproto-ref`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ originAtprotoRef: ref })
			});
			if (!res.ok) {
				error = (await res.text()).trim() || `Failed to update (${res.status}).`;
				return;
			}
			const result = await res.json();
			if (save.uri === saveUri) {
				onSaved(result.originAtprotoRef);
				open = false;
			}
			toast.success(ref ? 'AT Protocol source updated' : 'AT Protocol source removed');
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
			<Dialog.Title>AT Protocol source</Dialog.Title>
			<Dialog.Description>
				Identify the record this image was saved from. This applies only to this save. Leave both
				fields empty to remove the reference.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={submit} class="space-y-4">
			<div class="space-y-2">
				<Label for="origin-atproto-uri">Record URI</Label>
				<Input
					id="origin-atproto-uri"
					bind:value={uri}
					placeholder="at://did:plc:…/app.bsky.feed.post/…"
					disabled={submitting}
				/>
			</div>
			<div class="space-y-2">
				<Label for="origin-atproto-cid">Record CID</Label>
				<Input id="origin-atproto-cid" bind:value={cid} placeholder="bafy…" disabled={submitting} />
			</div>
			{#if error}
				<p role="alert" class="text-sm text-destructive">{error}</p>
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
				<Button type="submit" disabled={submitting}>{submitting ? 'Saving…' : 'Save'}</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
