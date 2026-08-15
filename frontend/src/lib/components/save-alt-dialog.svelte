<script lang="ts">
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { apiFetch } from '$lib/api';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Label } from '$lib/components/ui/label';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { getImageContent, type SaveView } from '$lib/types';

	interface Props {
		open: boolean;
		save: SaveView;
		onSaved: (alt: string) => void;
	}

	let { open = $bindable(), save, onSaved }: Props = $props();

	let alt = $state('');
	let submitting = $state(false);
	let error = $state<string | null>(null);

	$effect(() => {
		if (!open) return;
		untrack(() => {
			alt = getImageContent(save)?.alt ?? '';
			error = null;
		});
	});

	async function submit(e: Event) {
		e.preventDefault();
		const rkey = save.uri.split('/').pop();
		if (!rkey) {
			error = 'This save has no record key.';
			return;
		}
		submitting = true;
		error = null;
		try {
			// Empty is a valid submission — it clears the alt text.
			const next = alt.trim();
			const res = await apiFetch(`/api/save/${rkey}/alt`, {
				method: 'PUT',
				headers: {
					'Content-Type': 'application/x-www-form-urlencoded',
					Accept: 'application/json'
				},
				body: new URLSearchParams({ alt: next }).toString()
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
			onSaved(next);
			toast.success(next ? 'Alt text updated' : 'Alt text removed');
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
			<Dialog.Title>Alt text</Dialog.Title>
			<Dialog.Description>
				Describe the image for people using a screen reader. Unlike attribution, this applies only
				to this save — the same image saved elsewhere keeps its own description.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={submit} class="space-y-4">
			<div class="space-y-2">
				<Label for="save-alt-text">Description</Label>
				<Textarea
					id="save-alt-text"
					bind:value={alt}
					maxlength={2000}
					rows={4}
					disabled={submitting}
					placeholder="e.g. A red bicycle leaning against a green wall"
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
					{submitting ? 'Saving…' : 'Save'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
