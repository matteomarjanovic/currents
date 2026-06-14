<script lang="ts">
  import { untrack } from "svelte";
  import { clipper, type Collection } from "../../lib/clipper-store.svelte";
  import CollectionSelector from "../../lib/CollectionSelector.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Textarea } from "$lib/components/ui/textarea";
  import X from "@lucide/svelte/icons/x";

  const LOGIN_PAGE_URL =
    import.meta.env.VITE_LOGIN_PAGE_URL ?? "https://currents.is/oauth/login";

  type SaveState = "idle" | "saving" | "saved" | "error";
  let saveState = $state<SaveState>("idle");
  let errorMsg = $state("");
  // The user's explicit pick (null until they choose); otherwise we fall back
  // to the most-recently-used default, which updates as collections load in.
  let userPickedUri = $state<string | null>(null);
  let text = $state("");
  let attributionUrl = $state("");
  let attributionLicense = $state("");
  let attributionCredit = $state("");
  let showAttribution = $state(false);
  let pickerOpen = $state(false);

  // Auth polling
  let loginState = $state<"prompt" | "waiting">("prompt");
  let pollIntervalId: ReturnType<typeof setInterval> | null = null;

  function stopPolling() {
    if (pollIntervalId !== null) {
      clearInterval(pollIntervalId);
      pollIntervalId = null;
    }
  }

  // New collection creation
  let creatingCollection = $state(false);
  let createParent = $state<Collection | null>(null);
  let newCollectionName = $state("");
  let newCollectionDescription = $state("");
  let collectionError = $state("");

  // The collection that received the most recent save; on ties (a save in a
  // section also bumps its root) prefer the section, then the newest collection.
  function defaultCollectionUri(cols: Collection[]): string {
    const best = [...cols].sort((a, b) => {
      const ra = a.lastSavedAt ? Date.parse(a.lastSavedAt) : 0;
      const rb = b.lastSavedAt ? Date.parse(b.lastSavedAt) : 0;
      if (rb !== ra) return rb - ra;
      if (!!a.parentUri !== !!b.parentUri) return a.parentUri ? -1 : 1;
      const ca = a.createdAt ? Date.parse(a.createdAt) : 0;
      const cb = b.createdAt ? Date.parse(b.createdAt) : 0;
      return cb - ca;
    });
    return best[0]?.uri ?? "";
  }

  let selectedCollectionUri = $derived(
    userPickedUri ?? defaultCollectionUri(clipper.collections),
  );

  // Reset form state whenever a new image is shown. Keyed on imgUrl so that
  // collections arriving asynchronously don't wipe the form.
  $effect(() => {
    clipper.imgUrl;
    if (!clipper.visible) return;
    untrack(() => {
      saveState = "idle";
      errorMsg = "";
      text = "";
      attributionUrl = "";
      attributionLicense = "";
      attributionCredit = clipper.siteHints.attributionCredit ?? "";
      // Collapse the attribution form by default; expand it when the site
      // pre-filled a credit so the user sees what will be attributed.
      showAttribution = !!attributionCredit;
      loginState = "prompt";
      stopPolling();

      userPickedUri = null;
      creatingCollection = false;
      createParent = null;
      newCollectionName = "";
      newCollectionDescription = "";
      collectionError = "";
    });
  });

  // Clean up polling when dialog closes
  $effect(() => {
    if (!clipper.visible) stopPolling();
  });

  function handleLoginClick() {
    loginState = "waiting";
    pollIntervalId = setInterval(async () => {
      const res = await browser.runtime.sendMessage({ type: "CHECK_AUTH" });
      if (res.authenticated) {
        stopPolling();
        clipper.authState = "authenticated";
        clipper.userHandle = res.handle;
        clipper.collections = res.collections;
        clipper.collectionsLoading = false;
      }
    }, 3000);
  }

  function close() {
    document.dispatchEvent(new CustomEvent("currents-clipper-close"));
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape" && !pickerOpen) close();
  }

  function startCreate(parent: Collection | null) {
    createParent = parent;
    creatingCollection = true;
  }

  function cancelCreate() {
    creatingCollection = false;
    createParent = null;
    newCollectionName = "";
    newCollectionDescription = "";
    collectionError = "";
  }

  async function createCollection() {
    const name = newCollectionName.trim();
    if (!name) return;
    collectionError = "";
    try {
      const response = await browser.runtime.sendMessage({
        type: "CREATE_COLLECTION",
        name,
        description: newCollectionDescription.trim(),
        parent: createParent?.uri,
      });
      if (response.ok) {
        clipper.collections = [
          {
            uri: response.uri ?? "",
            name,
            saveCount: 0,
            parentUri: createParent?.uri,
            createdAt: new Date().toISOString(),
          },
          ...clipper.collections,
        ];
        userPickedUri = response.uri ?? clipper.collections[0]?.uri ?? "";
        cancelCreate();
      } else if (response.authError) {
        clipper.authState = "unauthenticated";
      } else {
        collectionError = response.error ?? "Failed to create collection";
      }
    } catch (e) {
      collectionError = String(e);
    }
  }

  async function save() {
    if (!selectedCollectionUri) return;
    saveState = "saving";
    try {
      const response = await browser.runtime.sendMessage({
        type: "SAVE_IMAGE",
        imgUrl: clipper.imgUrl,
        collectionUri: selectedCollectionUri,
        text: text.trim(),
        originUrl: clipper.originUrl,
        attributionUrl: attributionUrl.trim(),
        attributionLicense: attributionLicense.trim(),
        attributionCredit: attributionCredit.trim(),
      });
      if (response.ok) {
        saveState = "saved";
        setTimeout(close, 1500);
      } else if (response.authError) {
        clipper.authState = "unauthenticated";
      } else {
        saveState = "error";
        errorMsg = response.error ?? "Unknown error";
      }
    } catch (e) {
      saveState = "error";
      errorMsg = String(e);
    }
  }

  let busy = $derived(saveState === "saving" || saveState === "saved");

  // Promote the backdrop into the browser top layer so page UI can't paint
  // over it, whatever z-index (or top layer) the page uses. Falls back to the
  // shadow host's max z-index where the Popover API is unavailable.
  let backdrop = $state<HTMLDivElement | null>(null);
  $effect(() => {
    try {
      backdrop?.showPopover?.();
    } catch {
      // already shown
    }
  });
</script>

<svelte:window onkeydown={handleKeydown} />

{#if clipper.visible}
  <div
    bind:this={backdrop}
    popover="manual"
    class="fixed inset-0 isolate z-50 m-0 flex size-full items-center justify-center border-0 bg-black/30 p-0 font-sans"
    role="presentation"
    onclick={close}
  >
    <div
      class="relative flex w-90 max-w-[calc(100vw-3rem)] flex-col gap-3 rounded-4xl bg-popover p-6 text-sm text-popover-foreground shadow-xl ring-1 ring-foreground/5 dark:ring-foreground/10"
      role="dialog"
      aria-modal="true"
      aria-label="Save to Currents"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
      onmousedown={(e) => e.stopPropagation()}
      onpointerdown={(e) => e.stopPropagation()}
    >
      <Button
        variant="ghost"
        size="icon-sm"
        class="absolute top-3 right-3 text-muted-foreground"
        onclick={close}
        aria-label="Close"
      >
        <X />
      </Button>

      {#if clipper.authState === "unauthenticated"}
        {#if loginState === "waiting"}
          <div class="flex flex-col items-center gap-3 py-2">
            <div
              class="size-6 animate-spin rounded-full border-3 border-muted border-t-foreground"
            ></div>
            <p>Waiting for authentication…</p>
          </div>
        {:else}
          <p class="pr-6">
            <a
              class="text-primary underline underline-offset-4"
              href={LOGIN_PAGE_URL}
              target="_blank"
              rel="noreferrer"
              onclick={handleLoginClick}
            >
              Log in to Currents
            </a> to save images.
          </p>
        {/if}
      {:else}
        <img
          class="max-h-50 w-full rounded-2xl bg-muted object-contain"
          src={clipper.imgUrl}
          alt="Preview"
        />

        <div class="flex flex-col gap-1">
          <span class="text-xs text-muted-foreground">
            {#if creatingCollection && createParent}
              New section in {createParent.name}
            {:else if creatingCollection}
              New collection
            {:else}
              Collection
            {/if}
          </span>
          {#if creatingCollection}
            <div class="flex flex-col gap-2">
              <Input
                type="text"
                placeholder={createParent ? "Section name" : "Collection name"}
                bind:value={newCollectionName}
                onkeydown={(e) => {
                  if (e.key === "Enter") createCollection();
                }}
              />
              <Textarea
                placeholder="Description (optional)"
                rows={2}
                class="min-h-13"
                bind:value={newCollectionDescription}
              />
              <div class="flex gap-2">
                <Button variant="outline" class="flex-1" onclick={cancelCreate}
                  >Cancel</Button
                >
                <Button
                  class="flex-1"
                  onclick={createCollection}
                  disabled={!newCollectionName.trim()}
                >
                  Create
                </Button>
              </div>
              {#if collectionError}
                <p class="text-xs text-destructive">{collectionError}</p>
              {/if}
            </div>
          {:else}
            <CollectionSelector
              collections={clipper.collections}
              selectedUri={selectedCollectionUri}
              loading={clipper.collectionsLoading}
              disabled={busy}
              onSelect={(uri) => (userPickedUri = uri)}
              onCreate={startCreate}
              onOpenChange={(open) => (pickerOpen = open)}
            />
          {/if}
        </div>

        <Input
          type="text"
          placeholder="Add a note (optional)"
          bind:value={text}
          disabled={busy}
        />

        {#if showAttribution}
          <div class="flex flex-col gap-2">
            <span class="text-xs text-muted-foreground">Attribution</span>
            <Input
              type="text"
              placeholder="Credit (e.g. photographer name)"
              bind:value={attributionCredit}
              disabled={busy}
            />
            <Input
              type="url"
              placeholder="Source URL"
              bind:value={attributionUrl}
              disabled={busy}
            />
            <Input
              type="text"
              placeholder="License (e.g. CC BY 4.0)"
              bind:value={attributionLicense}
              disabled={busy}
            />
          </div>
        {:else}
          <Button
            variant="link"
            size="sm"
            class="h-auto justify-start self-start p-0 text-foreground"
            onclick={() => (showAttribution = true)}
            disabled={busy}
          >
            + Add attribution (recommended)
          </Button>
        {/if}

        {#if saveState === "saved"}
          <p class="text-center font-medium">Saved!</p>
        {:else}
          <Button
            onclick={save}
            disabled={!selectedCollectionUri ||
              creatingCollection ||
              saveState === "saving"}
          >
            {saveState === "saving" ? "Saving…" : "Save to Currents"}
          </Button>
          {#if saveState === "error"}
            <p class="text-xs text-destructive">{errorMsg}</p>
          {/if}
        {/if}
      {/if}
    </div>
  </div>
{/if}
