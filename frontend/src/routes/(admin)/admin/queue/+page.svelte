<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';

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

	// Map atproto reasonType + the [ai-generated] convention prefix to a short
	// label moderators can scan at a glance.
	function reasonLabel(reasonType?: string): string | null {
		if (!reasonType) return null;
		if (reasonType === 'sexual') return 'Sexual content';
		if (reasonType === 'violence') return 'Violent content';
		if (reasonType === 'ai-generated') return 'AI-generated';
		if (reasonType === 'other') return 'Other';
		return reasonType;
	}

	function reasonSnippet(reasonText?: string): string | null {
		if (!reasonText) return null;
		const stripped = reasonText.replace(/^\[ai-generated\]\s*/, '').trim();
		if (!stripped) return null;
		return stripped.length > 80 ? stripped.slice(0, 77) + '…' : stripped;
	}

	const limit = 50;

	// URL is the source of truth — derived values re-run load() on any navigation.
	let category = $derived(
		($page.url.searchParams.get('category') ?? '') as '' | 'nsfw' | 'violence' | 'other'
	);
	let order = $derived(
		($page.url.searchParams.get('order') ?? 'priority') as 'priority' | 'oldest' | 'newest'
	);
	let offset = $derived(Math.max(0, parseInt($page.url.searchParams.get('offset') ?? '0', 10)));

	let items = $state<Item[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	$effect(() => {
		void load(category, order, offset);
	});

	async function load(cat: string, ord: string, off: number) {
		loading = true;
		error = null;
		const params = new URLSearchParams();
		if (cat) params.set('category', cat);
		params.set('order', ord);
		params.set('limit', String(limit));
		params.set('offset', String(off));
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/admin/queue?${params}`, {
				credentials: 'include'
			});
			if (!res.ok) {
				error = `Failed to load queue (${res.status})`;
				items = [];
				return;
			}
			const data = (await res.json()) as { items: Item[] };
			items = data.items ?? [];
		} catch (e) {
			error = String(e);
			items = [];
		} finally {
			loading = false;
		}
	}

	function navigate(params: { category?: string; order?: string; offset?: number }) {
		const url = new URL($page.url);
		const cat = params.category ?? category;
		const ord = params.order ?? order;
		const off = params.offset ?? 0;
		if (cat) url.searchParams.set('category', cat);
		else url.searchParams.delete('category');
		url.searchParams.set('order', ord);
		if (off > 0) url.searchParams.set('offset', String(off));
		else url.searchParams.delete('offset');
		goto(url.toString());
	}

	function shortUri(uri: string): string {
		// at://did:plc:abc/coll/rkey → did:plc:abc/…/rkey
		const m = uri.match(/^at:\/\/([^/]+)\/[^/]+\/(.+)$/);
		if (!m) return uri;
		const did = m[1].length > 28 ? m[1].slice(0, 16) + '…' + m[1].slice(-6) : m[1];
		return `${did}/${m[2]}`;
	}

	function scoreBadgeClass(s?: number): string {
		if (s == null) return '';
		if (s >= 0.97) return 'bg-red-500/15 text-red-700 dark:text-red-300';
		if (s >= 0.85) return 'bg-orange-500/15 text-orange-700 dark:text-orange-300';
		return 'bg-yellow-500/15 text-yellow-700 dark:text-yellow-300';
	}
</script>

<svelte:head>
	<title>Queue · Admin · Currents</title>
</svelte:head>

<div class="flex flex-col gap-4">
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-xl font-semibold tracking-tight">Review queue</h1>
			<p class="text-sm text-muted-foreground">Pending items only.</p>
		</div>
		<div class="flex flex-wrap items-center gap-2 text-xs">
			<div class="flex items-center gap-1">
				<span class="text-muted-foreground">Category:</span>
				{#each [['', 'All'], ['nsfw', 'NSFW'], ['violence', 'Violence'], ['other', 'Other']] as [val, label] (val)}
					<Button
						size="sm"
						variant={category === val ? 'default' : 'outline'}
						onclick={() => navigate({ category: val })}
					>
						{label}
					</Button>
				{/each}
			</div>
			<div class="flex items-center gap-1">
				<span class="text-muted-foreground">Order by:</span>
				{#each [['priority', 'Priority'], ['oldest', 'Oldest'], ['newest', 'Newest']] as [val, label] (val)}
					<Button
						size="sm"
						variant={order === val ? 'default' : 'outline'}
						onclick={() => navigate({ order: val })}
					>
						{label}
					</Button>
				{/each}
			</div>
		</div>
	</header>

	{#if loading}
		<div class="py-10 text-center text-sm text-muted-foreground">Loading…</div>
	{:else if error}
		<div
			class="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
		>
			{error}
		</div>
	{:else if items.length === 0}
		<div class="py-10 text-center text-sm text-muted-foreground">No pending items.</div>
	{:else}
		<ul class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each items as item (item.id)}
				<li>
					<a
						href={`/admin/queue/${item.id}`}
						class="group flex items-stretch gap-3 overflow-hidden rounded-lg border border-border bg-card transition-colors hover:bg-muted/40"
					>
						<div class="relative h-28 w-28 flex-shrink-0 overflow-hidden bg-muted">
							{#if item.previewUrl}
								<img
									src={item.previewUrl}
									alt=""
									loading="lazy"
									class="h-full w-full object-cover blur-xs transition-[filter] group-hover:blur-none"
								/>
							{/if}
						</div>
						<div class="flex min-w-0 flex-1 flex-col gap-1 p-3">
							<div class="flex flex-wrap items-center gap-2 text-xs">
								<Badge variant="outline">{item.source}</Badge>
								{#if item.source === 'report'}
									{@const r = reasonLabel(item.reportReasonType)}
									{#if r}
										<Badge variant="secondary">{r}</Badge>
									{/if}
								{:else if item.category}
									<Badge variant="secondary">{item.category}</Badge>
								{/if}
								{#if item.priority >= 100}
									<Badge class="bg-red-500/15 text-red-700 dark:text-red-300">high</Badge>
								{/if}
								{#if item.disputed}
									<Badge class="bg-amber-500/15 text-amber-700 dark:text-amber-300">disputed</Badge>
								{/if}
							</div>
							{#if item.score != null}
								<div class="text-xs">
									<span
										class="inline-block rounded px-1.5 py-0.5 font-mono {scoreBadgeClass(
											item.score
										)}"
									>
										{item.score.toFixed(3)}
									</span>
								</div>
							{/if}
							{#if item.source === 'report'}
								{@const snippet = reasonSnippet(item.reportReasonText)}
								{#if snippet}
									<div
										class="line-clamp-2 text-xs text-muted-foreground"
										title={item.reportReasonText}
									>
										"{snippet}"
									</div>
								{/if}
							{/if}
							<div class="truncate font-mono text-xs text-muted-foreground" title={item.subjectUri}>
								{shortUri(item.subjectUri)}
							</div>
							<div class="mt-auto text-xs text-muted-foreground">
								{new Date(item.createdAt).toLocaleString()}
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
				onclick={() => navigate({ offset: offset - limit, category, order })}
			>
				← Previous
			</Button>
			<span class="text-xs text-muted-foreground">
				Showing {offset + 1}–{offset + items.length}
			</span>
			<Button
				size="sm"
				variant="outline"
				disabled={items.length < limit}
				onclick={() => navigate({ offset: offset + limit, category, order })}
			>
				Next →
			</Button>
		</div>
	{/if}
</div>
