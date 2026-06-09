<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';

	type Event = {
		id: number;
		actor: string;
		action: string;
		createdAt: string;
		subjectUri?: string;
		blobCid?: string;
		payload?: Record<string, unknown>;
	};

	let events = $state<Event[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let offset = $state(0);
	let busyId = $state<number | null>(null);
	const limit = 50;

	async function load() {
		loading = true;
		error = null;
		try {
			const res = await fetch(
				`${PUBLIC_APPVIEW_URL}/api/admin/me/events?limit=${limit}&offset=${offset}`,
				{ credentials: 'include' }
			);
			if (!res.ok) {
				error = `Failed to load (${res.status})`;
				events = [];
				return;
			}
			const data = (await res.json()) as { events: Event[] };
			events = data.events ?? [];
		} catch (e) {
			error = String(e);
			events = [];
		} finally {
			loading = false;
		}
	}

	onMount(load);

	function shortUri(uri: string): string {
		const m = uri.match(/^at:\/\/([^/]+)\/[^/]+\/(.+)$/);
		if (!m) return uri;
		const did = m[1].length > 28 ? m[1].slice(0, 16) + '…' + m[1].slice(-6) : m[1];
		return `${did}/${m[2]}`;
	}

	// Determine whether a row represents a label addition the moderator can undo.
	// `label_add` and `ai_flag` are the two action types that issue (non-negated)
	// labels; everything else (label_negate / takedown / acknowledge / self_label)
	// either is itself a negation or doesn't have a single-label undo path.
	function canNegate(e: Event): boolean {
		if (e.action !== 'label_add' && e.action !== 'ai_flag') return false;
		if (!e.subjectUri) return false;
		const val = (e.payload as { val?: string } | undefined)?.val;
		return typeof val === 'string' && val.length > 0;
	}

	async function negate(e: Event) {
		const val = (e.payload as { val?: string } | undefined)?.val;
		if (!e.subjectUri || !val) return;
		busyId = e.id;
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/admin/labels/negate`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					uri: e.subjectUri,
					val,
					blobCid: e.blobCid ?? ''
				})
			});
			if (!res.ok) {
				const text = await res.text();
				toast.error(`Negate failed: ${text || res.status}`);
				return;
			}
			toast.success(`Negated ${val}`);
			await load();
		} catch (err) {
			toast.error(String(err));
		} finally {
			busyId = null;
		}
	}

	function nextPage() {
		offset += limit;
		load();
	}

	function prevPage() {
		offset = Math.max(0, offset - limit);
		load();
	}

	function actionBadgeVariant(
		action: string
	): 'default' | 'secondary' | 'outline' | 'destructive' {
		switch (action) {
			case 'takedown':
				return 'destructive';
			case 'label_negate':
				return 'outline';
			case 'label_add':
			case 'ai_flag':
				return 'secondary';
			default:
				return 'outline';
		}
	}
</script>

<div class="flex flex-col gap-4">
	<header>
		<h1 class="text-xl font-semibold tracking-tight">My actions</h1>
		<p class="text-sm text-muted-foreground">
			Audit log of moderation actions you've taken, newest first. Use Negate to undo a label.
		</p>
	</header>

	{#if loading}
		<div class="py-10 text-center text-sm text-muted-foreground">Loading…</div>
	{:else if error}
		<div
			class="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
		>
			{error}
		</div>
	{:else if events.length === 0}
		<div class="py-10 text-center text-sm text-muted-foreground">
			No actions yet. Anything you do in /admin/queue will show up here.
		</div>
	{:else}
		<ul class="flex flex-col gap-2">
			{#each events as e (e.id)}
				{@const val = (e.payload as { val?: string } | undefined)?.val}
				<li class="flex flex-col gap-1 rounded-lg border border-border bg-card p-3 text-sm">
					<div class="flex flex-wrap items-center gap-2">
						<Badge variant={actionBadgeVariant(e.action)} class="font-mono">{e.action}</Badge>
						{#if val}
							<Badge variant="outline" class="font-mono">{val}</Badge>
						{/if}
						<span class="text-xs text-muted-foreground">
							{new Date(e.createdAt).toLocaleString()}
						</span>
						{#if canNegate(e)}
							<div class="ml-auto">
								<Button
									size="sm"
									variant="outline"
									onclick={() => negate(e)}
									disabled={busyId === e.id}
								>
									{busyId === e.id ? 'Negating…' : 'Negate'}
								</Button>
							</div>
						{/if}
					</div>
					{#if e.subjectUri}
						<div
							class="truncate font-mono text-xs text-muted-foreground"
							title={e.subjectUri}
						>
							{shortUri(e.subjectUri)}
						</div>
					{/if}
				</li>
			{/each}
		</ul>

		<div class="flex items-center justify-between pt-2">
			<Button size="sm" variant="outline" disabled={offset === 0} onclick={prevPage}>
				← Newer
			</Button>
			<span class="text-xs text-muted-foreground">
				Showing {offset + 1}–{offset + events.length}
			</span>
			<Button size="sm" variant="outline" disabled={events.length < limit} onclick={nextPage}>
				Older →
			</Button>
		</div>
	{/if}
</div>
