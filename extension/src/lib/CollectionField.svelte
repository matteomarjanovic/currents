<script lang="ts">
	import { clipper, type Collection } from './clipper-store.svelte';
	import CollectionSelector from './CollectionSelector.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';

	interface Props {
		// The collection currently targeted — the parent's `picked ?? default`.
		selectedUri: string;
		// The user's explicit choice, null until they make one. Bound so the parent
		// can keep falling back to the most-recently-used collection meanwhile
		// (collections arrive after the dialog opens, so the default moves).
		picked: string | null;
		// True while the inline "new collection" form is open; the parent disables
		// its save action so the two can't race.
		creating: boolean;
		disabled?: boolean;
		onOpenChange?: (open: boolean) => void;
	}

	let {
		selectedUri,
		picked = $bindable(null),
		creating = $bindable(false),
		disabled = false,
		onOpenChange
	}: Props = $props();

	let createParent = $state<Collection | null>(null);
	let newName = $state('');
	let newDescription = $state('');
	let error = $state('');

	function startCreate(parent: Collection | null) {
		createParent = parent;
		creating = true;
	}

	function cancelCreate() {
		creating = false;
		createParent = null;
		newName = '';
		newDescription = '';
		error = '';
	}

	async function createCollection() {
		const name = newName.trim();
		if (!name) return;
		error = '';
		try {
			const response = await browser.runtime.sendMessage({
				type: 'CREATE_COLLECTION',
				name,
				description: newDescription.trim(),
				parent: createParent?.uri
			});
			if (response.ok) {
				clipper.collections = [
					{
						uri: response.uri ?? '',
						name,
						saveCount: 0,
						parentUri: createParent?.uri,
						createdAt: new Date().toISOString()
					},
					...clipper.collections
				];
				picked = response.uri ?? clipper.collections[0]?.uri ?? '';
				cancelCreate();
			} else if (response.authError) {
				clipper.authState = 'unauthenticated';
			} else {
				error = response.error ?? 'Failed to create collection';
			}
		} catch (e) {
			error = String(e);
		}
	}
</script>

<div class="flex flex-col gap-1">
	<span class="text-xs text-muted-foreground">
		{#if creating && createParent}
			New section in {createParent.name}
		{:else if creating}
			New collection
		{:else}
			Collection
		{/if}
	</span>
	{#if creating}
		<div class="flex flex-col gap-2">
			<Input
				type="text"
				placeholder={createParent ? 'Section name' : 'Collection name'}
				bind:value={newName}
				onkeydown={(e) => {
					if (e.key === 'Enter') createCollection();
				}}
			/>
			<Textarea
				placeholder="Description (optional)"
				rows={2}
				class="min-h-13"
				bind:value={newDescription}
			/>
			<div class="flex gap-2">
				<Button variant="outline" class="flex-1" onclick={cancelCreate}>Cancel</Button>
				<Button class="flex-1" onclick={createCollection} disabled={!newName.trim()}>Create</Button>
			</div>
			{#if error}
				<p class="text-xs text-destructive">{error}</p>
			{/if}
		</div>
	{:else}
		<CollectionSelector
			collections={clipper.collections}
			{selectedUri}
			loading={clipper.collectionsLoading}
			{disabled}
			onSelect={(uri) => (picked = uri)}
			onCreate={startCreate}
			{onOpenChange}
		/>
	{/if}
</div>
