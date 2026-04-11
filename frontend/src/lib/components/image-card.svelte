<script lang="ts">
	import { untrack } from 'svelte';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import type { SaveView } from '$lib/types';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections, setLastUsedCollection } from '$lib/stores/collections.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';

	interface Props {
		item: SaveView;
	}

	let { item }: Props = $props();

	// Local optimistic copy of viewer saves — intentional one-time init, component is keyed by URI
	let localSaves = $state<{ collectionUri: string; saveUri: string }[]>(
		untrack(() => (item.viewer?.saves ? [...item.viewer.saves] : []))
	);

	let selectedCollectionUri = $state(
		untrack(() => localSaves[0]?.collectionUri ?? collections.lastUsedUri)
	);

	let dropdownOpen = $state(false);

	const OPTIMISTIC_URI = 'optimistic';

	function isSavedIn(uri: string): string | null {
		return localSaves.find((s) => s.collectionUri === uri)?.saveUri ?? null;
	}

	let selectedName = $derived(
		collections.items.find((c) => c.uri === selectedCollectionUri)?.name ?? 'Collection'
	);

	function isSavedInSelected() {
		return isSavedIn(selectedCollectionUri);
	}

	async function save(collectionUri: string) {
		localSaves = [...localSaves, { collectionUri, saveUri: OPTIMISTIC_URI }];
		setLastUsedCollection(collectionUri);
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/resave`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ saveUri: item.uri, collectionUri }),
				credentials: 'include'
			});
			if (!res.ok) throw new Error(`save: ${res.status}`);
			const data = await res.json();
			localSaves = localSaves.map((s) =>
				s.collectionUri === collectionUri && s.saveUri === OPTIMISTIC_URI
					? { collectionUri, saveUri: data.uri }
					: s
			);
		} catch (e) {
			console.error('save failed', e);
			localSaves = localSaves.filter(
				(s) => !(s.collectionUri === collectionUri && s.saveUri === OPTIMISTIC_URI)
			);
		}
	}

	async function unsave(saveUri: string, collectionUri: string) {
		const prev = localSaves;
		localSaves = localSaves.filter((s) => s.saveUri !== saveUri);
		try {
			const rkey = saveUri.split('/').pop()!;
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/save/${rkey}`, {
				method: 'DELETE',
				credentials: 'include'
			});
			if (!res.ok) throw new Error(`unsave: ${res.status}`);
		} catch (e) {
			console.error('unsave failed', e);
			localSaves = prev;
		}
	}

	function toggleCollection(uri: string) {
		selectedCollectionUri = uri;
		const existing = isSavedIn(uri);
		if (existing) {
			unsave(existing, uri);
		} else {
			save(uri);
		}
	}

	function handleButtonClick() {
		if (!selectedCollectionUri) return;
		const existing = isSavedIn(selectedCollectionUri);
		if (existing) {
			unsave(existing, selectedCollectionUri);
		} else {
			save(selectedCollectionUri);
		}
	}
</script>

<div class="group relative overflow-hidden rounded-lg">
	<img
		src={item.imageUrl}
		alt={item.text ?? ''}
		loading="lazy"
		class="w-full"
		style={item.width && item.height ? `aspect-ratio: ${item.width} / ${item.height}` : undefined}
	/>
	{#if auth.user && collections.loaded}
		<div
			class="absolute inset-0 flex flex-col justify-end bg-black/20 p-2 transition-opacity duration-300 {dropdownOpen ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}"
		>
			<div
				class="flex items-center gap-1.5 transition-transform duration-300 {dropdownOpen ? 'translate-y-0' : 'translate-y-2 group-hover:translate-y-0'}"
			>
				<DropdownMenu.Root bind:open={dropdownOpen}>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="secondary"
								size="sm"
								class="min-w-0 flex-1 justify-between truncate"
			
							>
								<span class="truncate">{selectedName}</span>
								<ChevronDown class="ml-1 size-3 shrink-0" />
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content>
						{#each collections.items as col (col.uri)}
							<DropdownMenu.Item onclick={() => toggleCollection(col.uri)}>
								{#if isSavedIn(col.uri)}
									<Check class="mr-2 size-4 shrink-0" />
								{:else}
									<div class="mr-2 size-4 shrink-0"></div>
								{/if}
								{col.name}
							</DropdownMenu.Item>
						{/each}
					</DropdownMenu.Content>
				</DropdownMenu.Root>

				<Button
					size="sm"
					variant={isSavedInSelected() ? 'secondary' : 'default'}
					onclick={handleButtonClick}
					disabled={!selectedCollectionUri}
				>
					{isSavedInSelected() ? 'Saved' : 'Save'}
				</Button>
			</div>
		</div>
	{/if}
</div>
