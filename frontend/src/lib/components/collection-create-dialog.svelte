<script lang="ts">
	import { apiFetch } from '$lib/api';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Label } from '$lib/components/ui/label';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import type { CollectionView } from '$lib/types';

	interface Props {
		open: boolean;
		onCreated: (collection: CollectionView) => void;
	}

	let { open = $bindable(), onCreated }: Props = $props();

	let name = $state('');
	let description = $state('');
	let submitting = $state(false);
	let error = $state<string | null>(null);

	$effect(() => {
		if (open) {
			name = '';
			description = '';
			error = null;
		}
	});

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
			const trimmedDescription = description.trim();
			const res = await apiFetch(`/collection`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/x-www-form-urlencoded',
					Accept: 'application/json'
				},
				body: new URLSearchParams({
					name: trimmedName,
					description: trimmedDescription
				}).toString()
			});
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
					open = false;
					return;
				}
				error = `Failed to create (${res.status}).`;
				return;
			}
			const data = (await res.json()) as { uri: string };
			const user = auth.user;
			const collection: CollectionView = {
				uri: data.uri,
				name: trimmedName,
				description: trimmedDescription || undefined,
				saveCount: 0,
				createdAt: new Date().toISOString(),
				author: user
					? {
							did: user.did,
							handle: user.handle,
							displayName: user.displayName,
							avatar: user.avatar
						}
					: undefined
			};
			onCreated(collection);
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
			<Dialog.Title>Create collection</Dialog.Title>
			<Dialog.Description
				>Give your new collection a name and optional description. IMPORTANT: Collections and saves
				are public for now.</Dialog.Description
			>
		</Dialog.Header>
		<form onsubmit={submit} class="space-y-4">
			<div class="space-y-2">
				<Label for="new-collection-name">Name</Label>
				<Input
					id="new-collection-name"
					bind:value={name}
					maxlength={100}
					required
					disabled={submitting}
				/>
			</div>
			<div class="space-y-2">
				<Label for="new-collection-description">Description</Label>
				<Textarea
					id="new-collection-description"
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
					{submitting ? 'Creating…' : 'Create'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
