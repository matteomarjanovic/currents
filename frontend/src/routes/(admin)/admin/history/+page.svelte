<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { apiFetch } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Input } from '$lib/components/ui/input';

	type Blob = {
		blobCid: string;
		latestAction: string;
		latestActor: string;
		latestEventAt: string;
		previewUrl?: string;
		sampleUri?: string;
	};

	const limit = 50;

	// URL is the source of truth — derived values re-run load() on any navigation.
	let q = $derived($page.url.searchParams.get('q') ?? '');
	let offset = $derived(Math.max(0, parseInt($page.url.searchParams.get('offset') ?? '0', 10)));

	// Local mirror of the search box: a writable $derived seeded from the URL, so
	// typing overrides it and any navigation re-seeds it. Debounced back into the URL.
	let search = $derived(q);

	let blobs = $state<Blob[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	$effect(() => {
		void load(q, offset);
	});

	async function load(query: string, off: number) {
		loading = true;
		error = null;
		const params = new URLSearchParams();
		if (query) params.set('q', query);
		params.set('limit', String(limit));
		params.set('offset', String(off));
		try {
			const res = await apiFetch(`/api/admin/history?${params}`);
			if (!res.ok) {
				error = `Failed to load history (${res.status})`;
				blobs = [];
				return;
			}
			const data = (await res.json()) as { blobs: Blob[] };
			blobs = data.blobs ?? [];
		} catch (e) {
			error = String(e);
			blobs = [];
		} finally {
			loading = false;
		}
	}

	function navigate(params: { q?: string; offset?: number }) {
		const url = new URL($page.url);
		const query = params.q ?? q;
		const off = params.offset ?? 0;
		if (query) url.searchParams.set('q', query);
		else url.searchParams.delete('q');
		if (off > 0) url.searchParams.set('offset', String(off));
		else url.searchParams.delete('offset');
		goto(url.toString());
	}

	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	function onSearchInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			if (search.trim() !== q) navigate({ q: search.trim() });
		}, 300);
	}

	function shortUri(uri: string): string {
		const m = uri.match(/^at:\/\/([^/]+)\/[^/]+\/(.+)$/);
		if (!m) return uri;
		const did = m[1].length > 28 ? m[1].slice(0, 16) + '…' + m[1].slice(-6) : m[1];
		return `${did}/${m[2]}`;
	}

	function actionLabel(action: string): string {
		switch (action) {
			case 'label_add':
				return 'label applied';
			case 'label_negate':
				return 'label negated';
			case 'ai_flag':
				return 'auto-flagged';
			case 'takedown':
				return 'taken down';
			case 'self_label':
				return 'self-labeled';
			case 'self_confirm':
				return 'owner confirmed';
			case 'self_dispute':
				return 'owner disputed';
			case 'owner_ignore':
				return 'owner ignored';
			default:
				return action;
		}
	}

	function actionBadgeClass(action: string): string {
		if (action === 'takedown') return 'bg-red-500/15 text-red-700 dark:text-red-300';
		if (action === 'label_negate') return 'bg-muted text-muted-foreground';
		return '';
	}
</script>

<svelte:head>
	<title>History · Admin · Currents</title>
</svelte:head>

<div class="flex flex-col gap-4">
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-xl font-semibold tracking-tight">History</h1>
			<p class="text-sm text-muted-foreground">
				Every image with moderation activity, most recent first.
			</p>
		</div>
		<Input
			type="search"
			placeholder="Search by blob CID or save URI…"
			bind:value={search}
			oninput={onSearchInput}
			class="h-9 w-full max-w-xs"
		/>
	</header>

	{#if loading}
		<div class="py-10 text-center text-sm text-muted-foreground">Loading…</div>
	{:else if error}
		<div
			class="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
		>
			{error}
		</div>
	{:else if blobs.length === 0}
		<div class="py-10 text-center text-sm text-muted-foreground">
			{q ? 'No matches.' : 'No moderation activity yet.'}
		</div>
	{:else}
		<ul class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each blobs as blob (blob.blobCid)}
				<li>
					<a
						href={`/admin/history/${blob.blobCid}`}
						class="group flex items-stretch gap-3 overflow-hidden rounded-lg border border-border bg-card transition-colors hover:bg-muted/40"
					>
						<div class="relative h-28 w-28 flex-shrink-0 overflow-hidden bg-muted">
							{#if blob.previewUrl}
								<img
									src={blob.previewUrl}
									alt=""
									loading="lazy"
									class="h-full w-full object-cover blur-xs transition-[filter] group-hover:blur-none"
								/>
							{/if}
						</div>
						<div class="flex min-w-0 flex-1 flex-col gap-1 p-3">
							<div class="flex flex-wrap items-center gap-2 text-xs">
								<Badge variant="secondary" class={actionBadgeClass(blob.latestAction)}>
									{actionLabel(blob.latestAction)}
								</Badge>
							</div>
							{#if blob.sampleUri}
								<div
									class="truncate font-mono text-xs text-muted-foreground"
									title={blob.sampleUri}
								>
									{shortUri(blob.sampleUri)}
								</div>
							{/if}
							<div class="truncate font-mono text-xs text-muted-foreground" title={blob.blobCid}>
								{blob.blobCid}
							</div>
							<div class="mt-auto text-xs text-muted-foreground">
								{new Date(blob.latestEventAt).toLocaleString()}
							</div>
						</div>
					</a>
				</li>
			{/each}
		</ul>

		<div class="flex items-center justify-between pt-2">
			<Button
				size="sm"
				variant="outline"
				disabled={offset === 0}
				onclick={() => navigate({ offset: offset - limit })}
			>
				← Previous
			</Button>
			<span class="text-xs text-muted-foreground">
				Showing {offset + 1}–{offset + blobs.length}
			</span>
			<Button
				size="sm"
				variant="outline"
				disabled={blobs.length < limit}
				onclick={() => navigate({ offset: offset + limit })}
			>
				Next →
			</Button>
		</div>
	{/if}
</div>
