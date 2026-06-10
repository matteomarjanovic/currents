<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';

	type Item = {
		id: number;
		source: string;
		subjectUri: string;
		blobCid?: string;
		category?: string;
		score?: number;
		priority: number;
		status: string;
		createdAt: string;
		previewUrl?: string;
		reportReasonType?: string;
		reportReasonText?: string;
		reportReporterDid?: string;
		disputed?: boolean;
		disputedAt?: string;
	};

	function reasonLabel(reasonType?: string): string | null {
		if (!reasonType) return null;
		if (reasonType === 'sexual') return 'Sexual content';
		if (reasonType === 'violence') return 'Violent content';
		if (reasonType === 'ai-generated') return 'AI-generated';
		if (reasonType === 'other') return 'Other';
		return reasonType;
	}

	function reasonBody(reasonText?: string): string {
		return reasonText?.trim() ?? '';
	}
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
		payload?: Record<string, unknown>;
	};
	type Detail = {
		item: Item;
		blobState?: BlobState;
		saves?: SaveSibling[];
		activeLabels?: string[];
		events?: Event[];
	};

	let id = $derived(page.params.id);
	let detail = $state<Detail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let takedownDialogOpen = $state(false);
	let takedownNotes = $state('');
	let applyLabelDialogOpen = $state(false);
	let applyLabelVal = $state<
		'porn' | 'sexual' | 'nudity' | 'graphic-media' | 'currents-ai-generated'
	>('porn');
	let submitting = $state(false);

	const APPLY_LABEL_OPTIONS: { val: 'porn' | 'sexual' | 'nudity' | 'graphic-media' | 'currents-ai-generated'; label: string }[] = [
		{ val: 'porn', label: 'porn — sexually explicit' },
		{ val: 'sexual', label: 'sexual — suggestive' },
		{ val: 'nudity', label: 'nudity — non-sexual' },
		{ val: 'graphic-media', label: 'graphic-media — violence / gore' },
		{ val: 'currents-ai-generated', label: 'currents-ai-generated' }
	];

	function defaultValForCategory(cat?: string): typeof applyLabelVal {
		if (cat === 'nsfw') return 'porn';
		if (cat === 'violence') return 'graphic-media';
		if (cat === 'ai-generated') return 'currents-ai-generated';
		return 'porn';
	}

	function openApplyLabel() {
		applyLabelVal = defaultValForCategory(detail?.item.category);
		applyLabelDialogOpen = true;
	}

	async function load() {
		loading = true;
		error = null;
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/admin/queue/${id}`, {
				credentials: 'include'
			});
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

	async function postAction(
		action: 'confirm' | 'takedown' | 'dismiss' | 'apply-label',
		body?: object
	) {
		if (!detail) return;
		submitting = true;
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/admin/queue/${id}/${action}`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: body ? JSON.stringify(body) : undefined
			});
			if (!res.ok) {
				const text = await res.text();
				toast.error(`${action} failed: ${text || res.status}`);
				return;
			}
			const data = (await res.json()) as { applied_to?: number };
			toast.success(
				data.applied_to != null
					? `${action} applied to ${data.applied_to} save${data.applied_to === 1 ? '' : 's'}`
					: `${action} complete`
			);
			takedownDialogOpen = false;
			applyLabelDialogOpen = false;
			goto('/admin/queue');
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

<div class="flex flex-col gap-4">
	<div class="flex items-center gap-2">
		<Button variant="ghost" size="sm" onclick={() => goto('/admin/queue')}>
			<ArrowLeft class="size-4" />
			Back to queue
		</Button>
	</div>

	{#if loading}
		<div class="py-10 text-center text-sm text-muted-foreground">Loading…</div>
	{:else if error}
		<div class="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
			{error}
		</div>
	{:else if detail}
		{@const item = detail.item}
		<div class="grid grid-cols-1 gap-6 md:grid-cols-[1fr_320px]">
			<!-- Preview -->
			<div class="overflow-hidden rounded-lg border border-border bg-card">
				{#if item.previewUrl}
					<img
						src={item.previewUrl}
						alt=""
						class="max-h-[70vh] w-full object-contain"
					/>
				{:else}
					<div class="flex h-64 items-center justify-center text-sm text-muted-foreground">
						No preview available
					</div>
				{/if}
			</div>

			<!-- Side panel -->
			<aside class="flex flex-col gap-4">
				<!-- Actions -->
				<div class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4">
					<h2 class="text-sm font-semibold">Actions</h2>
					{#if item.status !== 'pending'}
						<div class="text-xs text-muted-foreground">Already {item.status}.</div>
					{:else}
						<Button onclick={openApplyLabel} disabled={submitting}>Apply label…</Button>
						<Button
							variant="destructive"
							onclick={() => {
								takedownNotes = '';
								takedownDialogOpen = true;
							}}
							disabled={submitting}
						>
							Take down
						</Button>
						<Button
							variant="outline"
							onclick={() => postAction('dismiss')}
							disabled={submitting}
						>
							Dismiss
						</Button>
					{/if}
				</div>

				<!-- Report context — only when the item came from a user report. Shows the
				     full reason as submitted so moderators have the actual context for
				     triage instead of just the coarse `category='other'` bucket. -->
				{#if item.source === 'report'}
					{@const r = reasonLabel(item.reportReasonType)}
					{@const body = reasonBody(item.reportReasonText)}
					<div class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4 text-xs">
						<div class="flex items-center justify-between gap-2">
							<h2 class="text-sm font-semibold">Report</h2>
							{#if r}
								<Badge variant="secondary">{r}</Badge>
							{/if}
						</div>
						{#if item.reportReporterDid}
							<div class="text-muted-foreground">
								Reported by
								<span class="font-mono">{shortDid(item.reportReporterDid)}</span>
							</div>
						{/if}
						{#if body}
							<div class="whitespace-pre-wrap text-foreground/90">"{body}"</div>
						{:else}
							<div class="text-muted-foreground italic">No additional context provided.</div>
						{/if}
						{#if item.reportReasonType}
							<div class="text-muted-foreground">
								reasonType: <span class="font-mono">{item.reportReasonType}</span>
							</div>
						{/if}
					</div>
				{/if}

				<!-- Item meta -->
				<div class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4 text-xs">
					<div class="flex flex-wrap items-center gap-2">
						<Badge variant="outline">{item.source}</Badge>
						{#if item.category}
							<Badge variant="secondary">{item.category}</Badge>
						{/if}
						{#if item.priority >= 100}
							<Badge class="bg-red-500/15 text-red-700 dark:text-red-300">high priority</Badge>
						{/if}
						{#if item.disputed}
							<Badge class="bg-amber-500/15 text-amber-700 dark:text-amber-300">
								disputed by author
							</Badge>
						{/if}
					</div>
					<div class="text-muted-foreground">
						Created {new Date(item.createdAt).toLocaleString()}
					</div>
					{#if item.disputed && item.disputedAt}
						<div class="text-muted-foreground">
							Disputed {new Date(item.disputedAt).toLocaleString()}
						</div>
					{/if}
					<div class="break-all font-mono text-muted-foreground" title={item.subjectUri}>
						{item.subjectUri}
					</div>
					{#if item.blobCid}
						<div class="break-all font-mono text-muted-foreground">
							blob: {item.blobCid}
						</div>
					{/if}
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

				<!-- Active labels -->
				{#if detail.activeLabels && detail.activeLabels.length > 0}
					<div class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4 text-xs">
						<h2 class="text-sm font-semibold">Active labels</h2>
						<div class="flex flex-wrap gap-1">
							{#each detail.activeLabels as val (val)}
								<Badge variant="outline">{val}</Badge>
							{/each}
						</div>
					</div>
				{/if}
			</aside>
		</div>

		<!-- Sibling saves -->
		{#if detail.saves && detail.saves.length > 0}
			<section class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4">
				<h2 class="text-sm font-semibold">
					Saves sharing this blob ({detail.saves.length})
				</h2>
				<ul class="flex flex-col gap-1 text-xs">
					{#each detail.saves as s (s.uri)}
						<li class="flex items-center justify-between gap-2 truncate font-mono text-muted-foreground">
							<span class="truncate" title={s.uri}>{shortUri(s.uri)}</span>
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		<!-- Recent events -->
		{#if detail.events && detail.events.length > 0}
			<section class="flex flex-col gap-2 rounded-lg border border-border bg-card p-4">
				<h2 class="text-sm font-semibold">Audit log</h2>
				<ul class="flex flex-col gap-2 text-xs">
					{#each detail.events as e (e.id)}
						<li class="flex flex-col gap-0.5">
							<div class="flex items-center gap-2">
								<Badge variant="outline" class="font-mono">{e.action}</Badge>
								<span class="text-muted-foreground">
									{new Date(e.createdAt).toLocaleString()}
								</span>
							</div>
							<div class="font-mono text-muted-foreground">
								{e.actor === 'auto' ? 'auto' : shortDid(e.actor)}
								{#if e.payload}
									<span> · {JSON.stringify(e.payload)}</span>
								{/if}
							</div>
						</li>
					{/each}
				</ul>
			</section>
		{/if}
	{/if}
</div>

<!-- Takedown dialog: destructive, optional notes -->
<AlertDialog.Root bind:open={takedownDialogOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Take down content?</AlertDialog.Title>
			<AlertDialog.Description>
				This hides every save sharing the blob site-wide and issues a !hide label. Reversible via
				direct SQL, but should be treated as final.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<div class="flex flex-col gap-2">
			<Label for="takedown-notes" class="text-xs text-muted-foreground">Notes (optional)</Label>
			<Textarea id="takedown-notes" rows={3} bind:value={takedownNotes} />
		</div>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={submitting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action
				onclick={() => postAction('takedown', { notes: takedownNotes })}
				disabled={submitting}
				class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
			>
				Take down
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<!-- Apply Label dialog -->
<Dialog.Root bind:open={applyLabelDialogOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Apply a label</Dialog.Title>
			<Dialog.Description>
				The chosen label is issued on every save sharing this blob.
			</Dialog.Description>
		</Dialog.Header>
		<div class="flex flex-col gap-2">
			{#each APPLY_LABEL_OPTIONS as { val, label } (val)}
				<label class="flex items-center gap-2 rounded-md border border-border p-2 text-sm">
					<input
						type="radio"
						name="apply-label-val"
						value={val}
						checked={applyLabelVal === val}
						onchange={() => (applyLabelVal = val)}
					/>
					<span class="font-mono">{label}</span>
				</label>
			{/each}
		</div>
		<Dialog.Footer>
			<Button
				variant="outline"
				onclick={() => (applyLabelDialogOpen = false)}
				disabled={submitting}
			>
				Cancel
			</Button>
			<Button
				onclick={() => postAction('apply-label', { val: applyLabelVal })}
				disabled={submitting}
			>
				Apply
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
