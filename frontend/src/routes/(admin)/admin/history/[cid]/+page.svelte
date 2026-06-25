<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import { apiFetch } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Dialog from '$lib/components/ui/dialog';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';

	type BlobState = {
		harmState: string;
		aiGenerated: boolean;
		decidedBy?: string;
		decidedAt?: string;
		safetyScores?: { nsfw: number; violence: number; ai_generated: number };
		notes?: string;
	};
	type SaveSibling = { uri: string; authorDid: string };
	type Event = {
		id: number;
		actor: string;
		action: string;
		createdAt: string;
		subjectUri?: string;
		blobCid?: string;
		payload?: Record<string, unknown>;
	};
	type Detail = {
		blobCid: string;
		previewUrl?: string;
		blobState?: BlobState;
		saves?: SaveSibling[];
		activeLabels?: string[];
		events?: Event[];
	};

	// The canonical labels a moderator can toggle on a blob. Mirrors the backend
	// applyLabelAllowedVals set.
	const CANONICAL_LABELS: { val: string; label: string }[] = [
		{ val: 'porn', label: 'porn — sexually explicit' },
		{ val: 'sexual', label: 'sexual — suggestive' },
		{ val: 'nudity', label: 'nudity — non-sexual' },
		{ val: 'graphic-media', label: 'graphic-media — violence / gore' },
		{ val: 'currents-ai-generated', label: 'currents-ai-generated' }
	];

	let cid = $derived(page.params.cid);
	let detail = $state<Detail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let submitting = $state(false);
	let lightboxOpen = $state(false);

	let activeLabels = $derived(new Set(detail?.activeLabels ?? []));
	let canAct = $derived((detail?.saves?.length ?? 0) > 0);

	async function load() {
		loading = true;
		error = null;
		try {
			const res = await apiFetch(`/api/admin/blob/${cid}`);
			if (!res.ok) {
				error = res.status === 404 ? 'Not found' : `Load failed (${res.status})`;
				detail = null;
				return;
			}
			detail = (await res.json()) as Detail;
		} catch (e) {
			error = String(e);
			detail = null;
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function apply(val: string) {
		if (!detail) return;
		submitting = true;
		try {
			const res = await apiFetch(`/api/admin/labels/apply`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ blobCid: detail.blobCid, val })
			});
			if (!res.ok) {
				toast.error(`Apply failed: ${(await res.text()) || res.status}`);
				return;
			}
			const data = (await res.json()) as { applied_to?: number };
			toast.success(
				`Applied ${val} to ${data.applied_to ?? 0} save${data.applied_to === 1 ? '' : 's'}`
			);
			await load();
		} catch (e) {
			toast.error(String(e));
		} finally {
			submitting = false;
		}
	}

	async function negate(val: string) {
		if (!detail || !detail.saves?.length) return;
		submitting = true;
		try {
			const res = await apiFetch(`/api/admin/labels/negate`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ uri: detail.saves[0].uri, val, blobCid: detail.blobCid })
			});
			if (!res.ok) {
				toast.error(`Negate failed: ${(await res.text()) || res.status}`);
				return;
			}
			toast.success(`Negated ${val}`);
			await load();
		} catch (e) {
			toast.error(String(e));
		} finally {
			submitting = false;
		}
	}

	function shortDid(did: string): string {
		return did.length > 32 ? did.slice(0, 18) + '…' + did.slice(-8) : did;
	}

	function shortUri(uri: string): string {
		const m = uri.match(/^at:\/\/([^/]+)\/[^/]+\/(.+)$/);
		if (!m) return uri;
		return `${shortDid(m[1])}/${m[2]}`;
	}

	function scoreClass(s: number): string {
		if (s >= 0.97) return 'bg-red-500/15 text-red-700 dark:text-red-300';
		if (s >= 0.85) return 'bg-orange-500/15 text-orange-700 dark:text-orange-300';
		if (s >= 0.7) return 'bg-yellow-500/15 text-yellow-700 dark:text-yellow-300';
		return 'bg-muted text-muted-foreground';
	}
</script>

<svelte:head>
	<title>History · Admin · Currents</title>
</svelte:head>

<div class="flex flex-col gap-4">
	<div class="flex items-center gap-2">
		<Button variant="ghost" size="sm" onclick={() => goto('/admin/history')}>
			<ArrowLeft class="size-4" />
			Back to history
		</Button>
	</div>

	{#if loading}
		<div class="py-10 text-center text-sm text-muted-foreground">Loading…</div>
	{:else if error}
		<div
			class="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
		>
			{error}
		</div>
	{:else if detail}
		<div class="grid grid-cols-1 gap-6 md:grid-cols-[1fr_320px]">
			<!-- Preview (click to zoom) -->
			<div class="overflow-hidden rounded-lg border border-border bg-card">
				{#if detail.previewUrl}
					<button
						type="button"
						class="block w-full cursor-zoom-in"
						onclick={() => (lightboxOpen = true)}
					>
						<img src={detail.previewUrl} alt="" class="max-h-[70vh] w-full object-contain" />
					</button>
				{:else}
					<div class="flex h-64 items-center justify-center text-sm text-muted-foreground">
						No preview available
					</div>
				{/if}
			</div>

			<!-- Side panel -->
			<aside class="flex flex-col gap-4">
				<!-- Label controls -->
				<div class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4">
					<h2 class="text-sm font-semibold">Labels</h2>
					{#if !canAct}
						<div class="text-xs text-muted-foreground">
							No saves share this blob — nothing to label.
						</div>
					{/if}
					{#each CANONICAL_LABELS as { val, label } (val)}
						{@const applied = activeLabels.has(val)}
						<div class="flex items-center justify-between gap-2">
							<span class="truncate font-mono text-xs" title={label}>{val}</span>
							{#if applied}
								<Button
									size="sm"
									variant="outline"
									disabled={submitting || !canAct}
									onclick={() => negate(val)}
								>
									Negate
								</Button>
							{:else}
								<Button size="sm" disabled={submitting || !canAct} onclick={() => apply(val)}>
									Apply
								</Button>
							{/if}
						</div>
					{/each}
				</div>

				<!-- Meta -->
				<div class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4 text-xs">
					<div class="flex flex-wrap items-center gap-1">
						{#if detail.activeLabels && detail.activeLabels.length > 0}
							{#each detail.activeLabels as val (val)}
								<Badge variant="outline">{val}</Badge>
							{/each}
						{:else}
							<span class="text-muted-foreground">No active labels</span>
						{/if}
					</div>
					<div class="mt-1 font-mono break-all text-muted-foreground">
						blob: {detail.blobCid}
					</div>
				</div>

				<!-- Safety scores -->
				{#if detail.blobState?.safetyScores}
					{@const s = detail.blobState.safetyScores}
					<div class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4 text-xs">
						<h2 class="text-sm font-semibold">Safety scores</h2>
						{#each [['nsfw', s.nsfw], ['violence', s.violence], ['ai_generated', s.ai_generated]] as [axis, val] (axis)}
							<div class="flex items-center justify-between gap-2">
								<span class="text-muted-foreground">{axis}</span>
								<span class="rounded px-1.5 py-0.5 font-mono {scoreClass(val as number)}">
									{(val as number).toFixed(3)}
								</span>
							</div>
						{/each}
					</div>
				{/if}

				<!-- Blob state -->
				{#if detail.blobState}
					{@const bs = detail.blobState}
					<div class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4 text-xs">
						<h2 class="text-sm font-semibold">Blob state</h2>
						<div class="flex items-center justify-between">
							<span class="text-muted-foreground">harm</span>
							<Badge variant={bs.harmState === 'blocked' ? 'destructive' : 'secondary'}>
								{bs.harmState}
							</Badge>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-muted-foreground">ai-generated</span>
							<span>{bs.aiGenerated ? 'yes' : 'no'}</span>
						</div>
						{#if bs.decidedBy}
							<div class="text-muted-foreground">
								Decided by <span class="font-mono">{shortDid(bs.decidedBy)}</span>
								{#if bs.decidedAt}
									at {new Date(bs.decidedAt).toLocaleString()}
								{/if}
							</div>
						{/if}
						{#if bs.notes}
							<div class="mt-1 whitespace-pre-wrap text-muted-foreground">{bs.notes}</div>
						{/if}
					</div>
				{/if}
			</aside>
		</div>

		<!-- Sibling saves -->
		{#if detail.saves && detail.saves.length > 0}
			<section class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4">
				<h2 class="text-sm font-semibold">
					Saves sharing this image ({detail.saves.length})
				</h2>
				<ul class="flex flex-col gap-1 text-xs">
					{#each detail.saves as s (s.uri)}
						<li
							class="flex items-center justify-between gap-2 truncate font-mono text-muted-foreground"
						>
							<span class="truncate" title={s.uri}>{shortUri(s.uri)}</span>
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		<!-- Label history -->
		{#if detail.events && detail.events.length > 0}
			<section class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4">
				<h2 class="text-sm font-semibold">Label history</h2>
				<ul class="flex flex-col gap-2 text-xs">
					{#each detail.events as e (e.id)}
						{@const val = (e.payload as { val?: string } | undefined)?.val}
						<li class="flex flex-col gap-0.5">
							<div class="flex items-center gap-2">
								<Badge variant="outline" class="font-mono">{e.action}</Badge>
								{#if val}
									<Badge variant="outline" class="font-mono">{val}</Badge>
								{/if}
								<span class="text-muted-foreground">
									{new Date(e.createdAt).toLocaleString()}
								</span>
							</div>
							<div class="font-mono text-muted-foreground">
								{e.actor === 'auto' ? 'auto' : shortDid(e.actor)}
							</div>
						</li>
					{/each}
				</ul>
			</section>
		{/if}
	{/if}
</div>

<!-- Lightbox -->
<Dialog.Root bind:open={lightboxOpen}>
	<Dialog.Content class="max-w-[90vw] p-0 sm:max-w-[90vw]">
		{#if detail?.previewUrl}
			<img src={detail.previewUrl} alt="" class="max-h-[85vh] w-full object-contain" />
		{/if}
	</Dialog.Content>
</Dialog.Root>
