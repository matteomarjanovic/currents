<script lang="ts">
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Label } from '$lib/components/ui/label';
	import type { CollectionView } from '$lib/types';

	interface Props {
		open: boolean;
		collection: CollectionView;
		onSaved: (update: { name: string; description: string }) => void;
	}

	let { open = $bindable(), collection, onSaved }: Props = $props();

	let name = $state('');
	let description = $state('');
	let submitting = $state(false);
	let error = $state<string | null>(null);

	$effect(() => {
		if (open) {
			name = collection.name;
			description = collection.description ?? '';
			error = null;
		}
	});

	const rkey = $derived(collection.uri.split('/').pop() ?? '');

	async function submit(e: Event) {
		e.preventDefault();
		const trimmedName = name.trim();
		if (!trimmedName) {
			error = 'Name is required.';
			return;
		}
		submitting = true;
		error = null;
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/collection/${rkey}`, {
				method: 'PUT',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name: trimmedName, description: description.trim() })
			});
			if (!res.ok) {
				error = `Failed to save (${res.status}).`;
				return;
			}
			onSaved({ name: trimmedName, description: description.trim() });
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
			<Dialog.Title>Edit collection</Dialog.Title>
			<Dialog.Description
				>Update the name and description of your collection. IMPORTANT: Collections and saves are
				public for now.</Dialog.Description
			>
		</Dialog.Header>
		<form onsubmit={submit} class="space-y-4">
			<div class="space-y-2">
				<Label for="collection-name">Name</Label>
				<Input
					id="collection-name"
					bind:value={name}
					maxlength={100}
					required
					disabled={submitting}
				/>
			</div>
			<div class="space-y-2">
				<Label for="collection-description">Description</Label>
				<Textarea
					id="collection-description"
					bind:value={description}
					maxlength={1000}
					rows={4}
					disabled={submitting}
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
