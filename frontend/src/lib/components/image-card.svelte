<script lang="ts">
	import { untrack } from 'svelte';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import type { SaveView } from '$lib/types';
	import { auth } from '$lib/stores/auth.svelte';
	import { collections, setLastUsedCollection } from '$lib/stores/collections.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Toggle } from '$lib/components/ui/toggle';
	import * as Popover from '$lib/components/ui/popover';
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

	let sortedCollections = $derived(
		[...collections.items].sort((a, b) => {
			const aSaved = isSavedIn(a.uri) ? 0 : 1;
			const bSaved = isSavedIn(b.uri) ? 0 : 1;
			return aSaved - bSaved;
		})
	);

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
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
				}
				throw new Error(`save: ${res.status}`);
			}
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
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
				}
				throw new Error(`unsave: ${res.status}`);
			}
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
			class="absolute inset-0 flex flex-col justify-end bg-black/20 p-2 transition-opacity duration-300 {dropdownOpen
				? 'opacity-100'
				: 'opacity-0 group-hover:opacity-100'}"
		>
			<div
				class="flex items-center gap-1.5 transition-transform duration-300 {dropdownOpen
					? 'translate-y-0'
					: 'translate-y-2 group-hover:translate-y-0'}"
			>
				<Popover.Root bind:open={dropdownOpen}>
					<Popover.Trigger>
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
					</Popover.Trigger>
					<Popover.Content
						align="start"
						class="max-h-[40vh] gap-0 overflow-y-auto bg-popover/70 p-1.5 backdrop-blur-2xl backdrop-saturate-150"
					>
						{#each sortedCollections as col (col.uri)}
							<button
								class="flex w-full items-center gap-2.5 rounded-2xl px-3 py-2 text-sm hover:bg-foreground/10"
								onclick={() => toggleCollection(col.uri)}
							>
								{#if isSavedIn(col.uri)}
									<Check class="size-4 shrink-0" />
								{:else}
									<div class="size-4 shrink-0"></div>
								{/if}
								<span class="flex flex-col items-start truncate">
									<span class="truncate">{col.name}</span>
									<!-- All collections are currently public -->
									<span class="text-xs text-muted-foreground">Public</span>
								</span>
							</button>
						{/each}
					</Popover.Content>
				</Popover.Root>

				<Toggle
					size="sm"
					pressed={!!isSavedInSelected()}
					onPressedChange={handleButtonClick}
					disabled={!selectedCollectionUri}
					class="border border-transparent bg-primary text-primary-foreground hover:bg-primary/80 aria-pressed:bg-secondary aria-pressed:text-secondary-foreground aria-pressed:hover:bg-secondary/80"
				>
					{isSavedInSelected() ? 'Saved' : 'Save'}
				</Toggle>
			</div>
		</div>
	{:else if auth.checked}
		<div
			class="absolute inset-0 flex flex-col justify-end bg-black/20 p-2 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
		>
			<div
				class="flex translate-y-2 items-center justify-end gap-1.5 transition-transform duration-300 group-hover:translate-y-0"
			>
				<Button size="sm" variant="default" onclick={promptLogin}>Save</Button>
			</div>
		</div>
	{/if}
</div>
