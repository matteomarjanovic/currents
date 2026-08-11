<script lang="ts">
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { apiFetch } from '$lib/api';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';

	interface Props {
		open: boolean;
		// Distinct blob CIDs to attribute. PUT /save/attribution is keyed by blob and
		// fans out over every rkey of that blob, so one call per distinct blob covers
		// the whole selection.
		blobCids: string[];
		onDone: () => void;
	}

	let { open = $bindable(), blobCids, onDone }: Props = $props();

	let credit = $state('');
	let license = $state('');
	let url = $state('');
	let submitting = $state(false);
	let error = $state<string | null>(null);

	// Fresh fields each opening (the selection is mixed, so there's nothing to prefill).
	$effect(() => {
		if (!open) return;
		untrack(() => {
			credit = '';
			license = '';
			url = '';
			error = null;
		});
	});

	async function submit(e: Event) {
		e.preventDefault();
		if (blobCids.length === 0) return;
		submitting = true;
		error = null;
		let ok = 0;
		let failed = 0;
		let unauth = false;
		// Bounded concurrency: each call is a PDS write fan-out; too many at once
		// pressures the write budget.
		const queue = [...blobCids];
		async function worker() {
			for (;;) {
				const blobCid = queue.shift();
				if (!blobCid) return;
				try {
					const res = await apiFetch(`/save/attribution`, {
						method: 'PUT',
						headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
						body: new URLSearchParams({
							blob_cid: blobCid,
							attribution_url: url.trim(),
							attribution_license: license.trim(),
							attribution_credit: credit.trim()
						}).toString()
					});
					if (res.status === 401) {
						unauth = true;
						failed++;
					} else if (res.ok) {
						ok++;
					} else {
						failed++;
					}
				} catch {
					failed++;
				}
			}
		}
		await Promise.all([worker(), worker(), worker(), worker()]);
		submitting = false;
		if (unauth) {
			auth.user = null;
			open = false;
			promptLogin();
			return;
		}
		if (ok === 0) {
			error = 'Could not update attribution.';
			return;
		}
		toast.success(
			`Attribution applied to ${ok} image${ok === 1 ? '' : 's'}${failed > 0 ? ` · ${failed} failed` : ''}`
		);
		open = false;
		onDone();
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Attribution</Dialog.Title>
			<Dialog.Description>
				Credit the source of {blobCids.length}
				{blobCids.length === 1 ? 'image' : 'images'}. Applies to every collection of yours holding
				each image.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={submit} class="space-y-4">
			<div class="space-y-2">
				<Label for="bulk-attribution-credit">Credit</Label>
				<Input
					id="bulk-attribution-credit"
					bind:value={credit}
					maxlength={500}
					disabled={submitting}
					placeholder="e.g. Jane Doe"
				/>
			</div>
			<div class="space-y-2">
				<Label for="bulk-attribution-license">License</Label>
				<Input
					id="bulk-attribution-license"
					bind:value={license}
					maxlength={200}
					disabled={submitting}
					placeholder="e.g. CC BY 4.0"
				/>
			</div>
			<div class="space-y-2">
				<Label for="bulk-attribution-url">Attribution URL</Label>
				<Input
					id="bulk-attribution-url"
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
				<Button type="submit" disabled={submitting}>
					{submitting ? 'Applying…' : 'Apply'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
