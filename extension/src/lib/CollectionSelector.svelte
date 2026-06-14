<script lang="ts">
  import type { Collection } from './clipper-store.svelte';
  import { Button } from '$lib/components/ui/button';
  import * as Popover from '$lib/components/ui/popover';
  import Check from '@lucide/svelte/icons/check';
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import ChevronLeft from '@lucide/svelte/icons/chevron-left';
  import Plus from '@lucide/svelte/icons/plus';
  import FolderPlus from '@lucide/svelte/icons/folder-plus';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';

  interface Props {
    collections: Collection[];
    selectedUri: string;
    loading?: boolean;
    disabled?: boolean;
    onSelect: (uri: string) => void;
    onCreate: (parent: Collection | null) => void;
    onOpenChange?: (open: boolean) => void;
  }

  let {
    collections,
    selectedUri,
    loading = false,
    disabled = false,
    onSelect,
    onCreate,
    onOpenChange
  }: Props = $props();

  let open = $state(false);
  // When set, the list shows this collection's sections instead of the roots.
  let drillParent = $state<Collection | null>(null);
  $effect(() => {
    onOpenChange?.(open);
  });
  // Always start back at the top level when the picker reopens.
  $effect(() => {
    if (!open) drillParent = null;
  });

  // Most recently saved-into first; ties broken by newest collection.
  function byRecentSave(a: Collection, b: Collection): number {
    const ra = a.lastSavedAt ? Date.parse(a.lastSavedAt) : 0;
    const rb = b.lastSavedAt ? Date.parse(b.lastSavedAt) : 0;
    if (rb !== ra) return rb - ra;
    const ca = a.createdAt ? Date.parse(a.createdAt) : 0;
    const cb = b.createdAt ? Date.parse(b.createdAt) : 0;
    return cb - ca;
  }

  let childrenByParent = $derived.by(() => {
    const m = new Map<string, Collection[]>();
    for (const c of collections) {
      if (c.parentUri) {
        const arr = m.get(c.parentUri) ?? [];
        arr.push(c);
        m.set(c.parentUri, arr);
      }
    }
    return m;
  });
  let rootCollections = $derived(collections.filter((c) => !c.parentUri).sort(byRecentSave));
  let drillSections = $derived(
    drillParent ? [...(childrenByParent.get(drillParent.uri) ?? [])].sort(byRecentSave) : []
  );

  function sectionCount(uri: string): number {
    return childrenByParent.get(uri)?.length ?? 0;
  }

  // A root is "selected" if it or any of its sections is the current target.
  function isSelectedInTree(root: Collection): boolean {
    if (root.uri === selectedUri) return true;
    return (childrenByParent.get(root.uri) ?? []).some((c) => c.uri === selectedUri);
  }

  let selectedName = $derived(
    loading
      ? 'Loading collections…'
      : (collections.find((c) => c.uri === selectedUri)?.name ?? 'Select collection')
  );

  function pick(uri: string) {
    onSelect(uri);
    open = false;
  }

  function create(parent: Collection | null) {
    open = false;
    onCreate(parent);
  }
</script>

{#snippet preview(col: Collection)}
  {#if col.previewImages?.[0]}
    <img
      src={col.previewImages[0]}
      alt=""
      loading="lazy"
      class="size-9 shrink-0 rounded-md object-cover"
    />
  {:else}
    <div class="size-9 shrink-0 rounded-md bg-muted"></div>
  {/if}
{/snippet}

<!-- A leaf row: clicking it selects the collection. -->
{#snippet pickRow(col: Collection, subtitle: string, bold: boolean)}
  <button
    class="flex w-full items-center gap-2.5 rounded-2xl px-2 py-1.5 text-sm hover:bg-foreground/10"
    onclick={() => pick(col.uri)}
  >
    {@render preview(col)}
    <span class="flex flex-1 flex-col items-start truncate">
      <span class="truncate {bold ? 'font-medium' : ''}">{col.name}</span>
      <span class="text-xs text-muted-foreground">{subtitle}</span>
    </span>
    {#if col.uri === selectedUri}
      <Check class="size-4 shrink-0" />
    {/if}
  </button>
{/snippet}

<!-- A collection with sections: clicking it drills into its sections. -->
{#snippet navRow(root: Collection)}
  {@const n = sectionCount(root.uri)}
  <button
    class="flex w-full items-center gap-2.5 rounded-2xl px-2 py-1.5 text-sm hover:bg-foreground/10"
    onclick={() => (drillParent = root)}
  >
    {@render preview(root)}
    <span class="flex flex-1 flex-col items-start truncate">
      <span class="truncate">{root.name}</span>
      <span class="text-xs text-muted-foreground">
        Public • {n}
        {n === 1 ? 'section' : 'sections'}
      </span>
    </span>
    {#if isSelectedInTree(root)}
      <Check class="size-4 shrink-0 text-muted-foreground" />
    {/if}
    <ChevronRight class="size-4 shrink-0 text-muted-foreground" />
  </button>
{/snippet}

<Popover.Root bind:open>
  <Popover.Trigger disabled={disabled || loading}>
    {#snippet child({ props })}
      <Button
        {...props}
        variant="secondary"
        class="w-full min-w-0 justify-between"
        disabled={disabled || loading}
      >
        <span class="truncate {loading ? 'text-muted-foreground' : ''}">{selectedName}</span>
        {#if loading}
          <LoaderCircle class="ml-1 size-3 shrink-0 animate-spin" />
        {:else}
          <ChevronDown class="ml-1 size-3 shrink-0" />
        {/if}
      </Button>
    {/snippet}
  </Popover.Trigger>
  <Popover.Content
    align="start"
    portalProps={{ disabled: true }}
    class="scrollbar-hide max-h-[40vh] gap-0 overflow-y-auto bg-popover/70 p-1.5 backdrop-blur-2xl backdrop-saturate-150"
  >
    {#if drillParent}
      {@const dp = drillParent}
      <button
        class="flex w-full items-center gap-1.5 rounded-2xl px-2 py-1.5 text-sm font-medium hover:bg-foreground/10"
        onclick={() => (drillParent = null)}
      >
        <ChevronLeft class="size-4 shrink-0" />
        <span class="truncate">All collections</span>
      </button>
      {@render pickRow(dp, 'Whole collection', true)}
      <!-- Sections -->
      <span class="block px-2 pt-3 text-xs text-muted-foreground">Sections</span>
      <button
        class="flex w-full items-center gap-2.5 rounded-2xl px-3 py-2 text-sm hover:bg-foreground/10"
        onclick={() => create(dp)}
      >
        <FolderPlus class="size-4 shrink-0" />
        <span class="truncate">Create section</span>
      </button>
      {#each drillSections as sec (sec.uri)}
        {@render pickRow(sec, 'Public', false)}
      {/each}
    {:else}
      <button
        class="flex w-full items-center gap-2.5 rounded-2xl px-3 py-2 text-sm hover:bg-foreground/10"
        onclick={() => create(null)}
      >
        <Plus class="size-4 shrink-0" />
        <span class="truncate">Create new collection</span>
      </button>
      {#each rootCollections as root (root.uri)}
        {#if sectionCount(root.uri) > 0}
          {@render navRow(root)}
        {:else}
          {@render pickRow(root, 'Public', false)}
        {/if}
      {/each}
    {/if}
  </Popover.Content>
</Popover.Root>
