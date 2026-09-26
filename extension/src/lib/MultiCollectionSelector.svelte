<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { clipper, type Collection } from './clipper-store.svelte';
	import { orderedRoots, orderedSections } from './collection-order';
	import { scrollFade } from './scroll-fade';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import Check from '@lucide/svelte/icons/check';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Plus from '@lucide/svelte/icons/plus';
	import User from '@lucide/svelte/icons/user';

	interface Props {
		selected: SvelteSet<string>;
		creating: boolean;
		disabled?: boolean;
	}
	let { selected, creating = $bindable(false), disabled = false }: Props = $props();
	let parent = $state<Collection | null>(null);
	let createParent = $state<Collection | null>(null);
	let name = $state('');
	let description = $state('');
	let error = $state('');
	let submitting = $state(false);
	let roots = $derived(orderedRoots(clipper.collections, clipper.lastUsedCollectionUri));
	let sections = $derived(
		parent ? orderedSections(clipper.collections, parent.uri, clipper.lastUsedCollectionUri) : []
	);

	function toggle(uri: string) {
		if (!selected.delete(uri)) selected.add(uri);
	}

	function cancelCreate() {
		creating = false;
		createParent = null;
		name = '';
		description = '';
		error = '';
	}

	async function createCollection() {
		if (!name.trim() || submitting || disabled) return;
		submitting = true;
		error = '';
		try {
			const res = await browser.runtime.sendMessage({
				type: 'CREATE_COLLECTION',
				name: name.trim(),
				description: description.trim(),
				parent: createParent?.uri
			});
			if (res.ok && res.uri) {
				clipper.collections = [
					{
						uri: res.uri,
						name: name.trim(),
						saveCount: 0,
						parentUri: createParent?.uri,
						createdAt: new Date().toISOString()
					},
					...clipper.collections
				];
				selected.add(res.uri);
				if (!createParent) parent = null;
				cancelCreate();
			} else if (res.authError) {
				clipper.authState = 'unauthenticated';
			} else {
				error = res.error ?? 'Failed to create collection';
			}
		} catch (e) {
			error = String(e);
		} finally {
			submitting = false;
		}
	}
</script>

{#snippet preview(collection: Collection)}
	{#if collection.previews?.[0]}
		<img
			src={collection.previews[0].url}
			alt=""
			loading="lazy"
			class="size-10 shrink-0 rounded-lg object-cover"
		/>
	{:else}
		<div class="size-10 shrink-0 rounded-lg bg-muted"></div>
	{/if}
{/snippet}

{#snippet row(collection: Collection, subtitle: string)}
	<button
		type="button"
		class="flex w-full items-center gap-3 rounded-xl p-2 text-left hover:bg-muted"
		aria-pressed={selected.has(collection.uri)}
		{disabled}
		onclick={() => toggle(collection.uri)}
	>
		{@render preview(collection)}
		<span class="min-w-0 flex-1">
			<span class="block truncate font-medium">{collection.name}</span>
			<span class="block text-xs text-muted-foreground">{subtitle}</span>
		</span>
		<span
			class="mr-1 grid size-5 shrink-0 place-items-center rounded-full border {selected.has(
				collection.uri
			)
				? 'border-foreground bg-foreground text-background'
				: 'border-border'}"
		>
			{#if selected.has(collection.uri)}<Check class="size-3.5" />{/if}
		</span>
	</button>
{/snippet}

<div use:scrollFade class="min-h-0 flex-1 overflow-y-auto">
	{#if creating}
		<div class="flex flex-col gap-2 pt-1">
			<p class="text-base font-semibold">New {createParent ? 'section' : 'collection'}</p>
			<Input
				placeholder={createParent ? 'Section name' : 'Collection name'}
				{disabled}
				bind:value={name}
				onkeydown={(e) => {
					if (e.key === 'Enter') void createCollection();
				}}
			/>
			<Textarea placeholder="Description (optional)" bind:value={description} rows={2} {disabled} />
			{#if error}<p class="text-xs text-destructive">{error}</p>{/if}
			<div class="flex gap-2">
				<Button variant="outline" class="flex-1" onclick={cancelCreate}>Cancel</Button>
				<Button
					class="flex-1"
					onclick={createCollection}
					disabled={!name.trim() || submitting || disabled}>Create</Button
				>
			</div>
		</div>
	{:else if parent}
		<button
			type="button"
			class="mb-2 flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
			onclick={() => (parent = null)}
			{disabled}
		>
			<ChevronLeft class="size-4" /> All collections
		</button>
		{@render row(parent, 'Whole collection')}
		<p class="px-2 pt-3 pb-1 text-xs text-muted-foreground">Sections</p>
		{#each sections as section (section.uri)}
			{@render row(section, 'Section')}
		{/each}
		<button
			type="button"
			class="flex w-full items-center gap-3 rounded-xl p-2 text-left hover:bg-muted"
			onclick={() => {
				createParent = parent;
				creating = true;
			}}
			{disabled}
		>
			<span class="grid size-10 place-items-center rounded-lg bg-muted"
				><Plus class="size-4" /></span
			>
			<span>Create section</span>
		</button>
	{:else}
		<button
			type="button"
			class="flex w-full items-center gap-3 rounded-xl p-2 text-left hover:bg-muted"
			aria-pressed={selected.has('')}
			{disabled}
			onclick={() => toggle('')}
		>
			<span class="grid size-10 place-items-center rounded-lg bg-muted"
				><User class="size-4" /></span
			>
			<span class="min-w-0 flex-1"
				><span class="block font-medium">Profile</span><span
					class="block text-xs text-muted-foreground">Save without a collection</span
				></span
			>
			<span
				class="mr-1 grid size-5 shrink-0 place-items-center rounded-full border {selected.has('')
					? 'border-foreground bg-foreground text-background'
					: 'border-border'}"
				>{#if selected.has('')}<Check class="size-3.5" />{/if}</span
			>
		</button>
		{#if clipper.collectionsLoading}<p class="p-3 text-sm text-muted-foreground">
				Loading collections…
			</p>{/if}
		{#each roots as collection (collection.uri)}
			{#if collection.parentUri || !clipper.collections.some((item) => item.parentUri === collection.uri)}
				{@render row(
					collection,
					collection.parentUri
						? `Section in ${clipper.collections.find((item) => item.uri === collection.parentUri)?.name ?? 'collection'}`
						: 'Collection'
				)}
			{:else}
				<button
					type="button"
					class="flex w-full items-center gap-3 rounded-xl p-2 text-left hover:bg-muted"
					onclick={() => (parent = collection)}
					{disabled}
				>
					{@render preview(collection)}
					<span class="min-w-0 flex-1 truncate font-medium">{collection.name}</span>
					{#if selected.has(collection.uri) || clipper.collections.some((item) => item.parentUri === collection.uri && selected.has(item.uri))}
						<span
							class="grid size-5 shrink-0 place-items-center rounded-full border border-foreground/50 text-foreground"
							><Check class="size-3.5" /></span
						>
					{/if}
					<ChevronRight class="size-4 text-muted-foreground" />
				</button>
			{/if}
		{/each}
	{/if}
</div>
