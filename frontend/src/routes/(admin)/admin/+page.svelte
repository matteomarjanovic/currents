<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Activity,
		Clock3,
		Database,
		HardDrive,
		RefreshCw,
		Server,
		Sparkles
	} from '@lucide/svelte';
	import { apiFetch } from '$lib/api';
	import * as Button from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';

	type Queue = { pending: number; max: number };
	type Job = {
		job: string;
		status: 'success' | 'failed';
		startedAt: string;
		finishedAt: string;
		details: Record<string, unknown>;
	};
	type Container = { Name: string; CPUPerc: string; MemUsage: string };
	type HostPayload = {
		memory: { totalBytes: number; availableBytes: number };
		load1: number;
		storage: { path: string; totalBytes: number; usedBytes: number; availableBytes: number };
		containers: Container[];
		modelVersion: string | null;
		modelUpdatedAt: string | null;
	};
	type Host = { host: 'main' | 'inference'; reportedAt: string; payload: HostPayload };
	type Overview = {
		now: string;
		appview: {
			heapBytes: number;
			systemBytes: number;
			goroutines: number;
			pool: {
				acquiredConns: number;
				idleConns: number;
				totalConns: number;
				maxConns: number;
				emptyAcquireCount: number;
			};
		};
		database: {
			sizeBytes: number;
			connectionCount: number;
			maxConnections: number;
			pendingReview: number;
			largestTables: { name: string; bytes: number }[];
		};
		inference: {
			available: boolean;
			health?: {
				device: string;
				model: string;
				umap: boolean;
				queues: { text: Queue; image: Queue };
			};
			error?: string;
		};
		background: {
			missingVisualIdentityCount: number;
			distinctMissingBlobCidCount: number;
			collectionsMissingEmbeddingCount: number;
			oldestMissingAgeSec?: number;
		};
		jobs: Job[];
		hosts: Host[];
	};

	let overview = $state<Overview | null>(null);
	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<string | null>(null);

	function bytes(value: number): string {
		if (!Number.isFinite(value)) return '—';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let unit = 0;
		let size = value;
		while (size >= 1000 && unit < units.length - 1) {
			size /= 1000;
			unit++;
		}
		return `${size >= 10 || unit === 0 ? size.toFixed(0) : size.toFixed(1)} ${units[unit]}`;
	}

	function ago(value: string | undefined): string {
		if (!value) return 'Not recorded yet';
		const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000));
		if (seconds < 60) return 'just now';
		if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
		if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
		return `${Math.floor(seconds / 86400)}d ago`;
	}

	function date(value: string): string {
		return new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	}

	function ratio(value: number, total: number): string {
		return total ? `${Math.round((value / total) * 100)}%` : '—';
	}

	function host(name: Host['host']): Host | undefined {
		return overview?.hosts.find((item) => item.host === name);
	}

	function job(name: string): Job | undefined {
		return overview?.jobs.find((item) => item.job === name);
	}

	function stale(reportedAt: string): boolean {
		return Date.now() - new Date(reportedAt).getTime() > 3 * 60 * 1000;
	}

	async function load(manual = false) {
		if (manual) refreshing = true;
		try {
			const res = await apiFetch('/api/admin/overview');
			if (!res.ok) throw new Error(`Could not load the dashboard (${res.status})`);
			overview = (await res.json()) as Overview;
			error = null;
		} catch (e) {
			error = String(e);
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	onMount(() => {
		load();
		const timer = window.setInterval(() => load(), 30_000);
		return () => window.clearInterval(timer);
	});
</script>

<svelte:head>
	<title>Admin · Currents</title>
</svelte:head>

<div class="flex flex-col gap-6">
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-xl font-semibold tracking-tight">Operations</h1>
			<p class="text-sm text-muted-foreground">
				Live service, capacity, and scheduled-work status.
			</p>
		</div>
		<div class="flex items-center gap-3">
			{#if overview}
				<span class="text-xs text-muted-foreground">Updated {ago(overview.now)}</span>
			{/if}
			<Button.Root variant="outline" size="sm" onclick={() => load(true)} disabled={refreshing}>
				<RefreshCw class="size-3.5 {refreshing ? 'animate-spin' : ''}" />
				Refresh
			</Button.Root>
		</div>
	</header>

	{#if loading}
		<div class="py-12 text-center text-sm text-muted-foreground">Loading operations status…</div>
	{:else if !overview}
		<div
			class="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
		>
			{error ?? 'The dashboard is unavailable.'}
		</div>
	{:else}
		{#if error}
			<div
				class="rounded-md border border-amber-500/30 bg-amber-500/5 p-3 text-sm text-amber-700 dark:text-amber-400"
			>
				Showing the last successful response. {error}
			</div>
		{/if}

		<section class="grid gap-4 md:grid-cols-3">
			<Card.Root>
				<Card.Header class="pb-3">
					<Card.Description class="flex items-center gap-2"
						><Server class="size-4" />Appview</Card.Description
					>
					<Card.Title class="text-lg">Running</Card.Title>
				</Card.Header>
				<Card.Content class="grid gap-2 text-sm">
					<div class="flex justify-between">
						<span class="text-muted-foreground">Heap</span><span
							>{bytes(overview.appview.heapBytes)}</span
						>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Goroutines</span><span
							>{overview.appview.goroutines}</span
						>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">DB pool</span><span
							>{overview.appview.pool.acquiredConns}/{overview.appview.pool.maxConns} acquired</span
						>
					</div>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="pb-3">
					<Card.Description class="flex items-center gap-2"
						><Database class="size-4" />PostgreSQL</Card.Description
					>
					<Card.Title class="text-lg">{bytes(overview.database.sizeBytes)}</Card.Title>
				</Card.Header>
				<Card.Content class="grid gap-2 text-sm">
					<div class="flex justify-between">
						<span class="text-muted-foreground">Connections</span><span
							>{overview.database.connectionCount}/{overview.database.maxConnections}</span
						>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Review queue</span><a
							class="underline underline-offset-2"
							href="/moderation/queue">{overview.database.pendingReview}</a
						>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Data volume</span><span
							>{host('main')
								? ratio(
										host('main')!.payload.storage.usedBytes,
										host('main')!.payload.storage.totalBytes
									)
								: 'Waiting for host report'}</span
						>
					</div>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="pb-3">
					<Card.Description class="flex items-center gap-2"
						><Sparkles class="size-4" />Inference</Card.Description
					>
					<Card.Title class="text-lg"
						>{overview.inference.available ? 'Reachable' : 'Unavailable'}</Card.Title
					>
				</Card.Header>
				<Card.Content class="grid gap-2 text-sm">
					{#if overview.inference.health}
						<div class="flex justify-between">
							<span class="text-muted-foreground">Image queue</span><span
								>{overview.inference.health.queues.image.pending}/{overview.inference.health.queues
									.image.max}</span
							>
						</div>
						<div class="flex justify-between">
							<span class="text-muted-foreground">Text queue</span><span
								>{overview.inference.health.queues.text.pending}/{overview.inference.health.queues
									.text.max}</span
							>
						</div>
						<div class="flex justify-between">
							<span class="text-muted-foreground">UMAP</span><span
								>{overview.inference.health.umap ? 'Loaded' : 'Missing'}</span
							>
						</div>
					{:else}
						<p class="text-muted-foreground">{overview.inference.error ?? 'No health response.'}</p>
					{/if}
				</Card.Content>
			</Card.Root>
		</section>

		<section class="grid gap-4 lg:grid-cols-2">
			<Card.Root>
				<Card.Header>
					<Card.Title class="flex items-center gap-2"
						><Activity class="size-4" />Data pipeline</Card.Title
					>
					<Card.Description>Items waiting for enrichment or collection embedding.</Card.Description>
				</Card.Header>
				<Card.Content class="grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
					<div>
						<p class="text-2xl font-semibold tabular-nums">
							{overview.background.missingVisualIdentityCount}
						</p>
						<p class="text-muted-foreground">Missing identities</p>
					</div>
					<div>
						<p class="text-2xl font-semibold tabular-nums">
							{overview.background.distinctMissingBlobCidCount}
						</p>
						<p class="text-muted-foreground">Distinct blobs</p>
					</div>
					<div>
						<p class="text-2xl font-semibold tabular-nums">
							{overview.background.collectionsMissingEmbeddingCount}
						</p>
						<p class="text-muted-foreground">Collections</p>
					</div>
					<div>
						<p class="text-2xl font-semibold tabular-nums">
							{overview.background.oldestMissingAgeSec == null
								? '—'
								: ago(
										new Date(
											Date.now() - overview.background.oldestMissingAgeSec * 1000
										).toISOString()
									)}
						</p>
						<p class="text-muted-foreground">Oldest item</p>
					</div>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header>
					<Card.Title class="flex items-center gap-2"
						><Clock3 class="size-4" />Scheduled work</Card.Title
					>
					<Card.Description>Most recent terminal run for each job.</Card.Description>
				</Card.Header>
				<Card.Content class="grid gap-3 text-sm">
					{#each [{ key: 'postgres_backup', label: 'PostgreSQL backup' }, { key: 'umap_train', label: 'UMAP training' }, { key: 'clustering', label: 'Daily clustering' }] as item (item.key)}
						{@const run = job(item.key)}
						<div class="flex items-center justify-between gap-3">
							<div>
								<p>{item.label}</p>
								<p class="text-xs text-muted-foreground">
									{run ? `${ago(run.finishedAt)} · ${date(run.finishedAt)}` : 'Not recorded yet'}
								</p>
							</div>
							<span
								class="rounded-full px-2 py-0.5 text-xs {run?.status === 'failed'
									? 'bg-destructive/10 text-destructive'
									: run
										? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
										: 'bg-muted text-muted-foreground'}">{run?.status ?? 'unknown'}</span
							>
						</div>
					{/each}
				</Card.Content>
			</Card.Root>
		</section>

		<section class="grid gap-4 lg:grid-cols-2">
			{#each ['main', 'inference'] as name (name)}
				{@const snapshot = host(name as Host['host'])}
				<Card.Root>
					<Card.Header>
						<Card.Title class="flex items-center gap-2"
							><HardDrive class="size-4" />{name === 'main'
								? 'Main VM'
								: 'Inference VM'}</Card.Title
						>
						<Card.Description
							>{snapshot
								? `${stale(snapshot.reportedAt) ? 'Stale report' : 'Reported'} ${ago(snapshot.reportedAt)}`
								: 'Waiting for the host reporter.'}</Card.Description
						>
					</Card.Header>
					{#if snapshot}
						<Card.Content class="grid gap-4">
							<div class="grid grid-cols-3 gap-3 text-sm">
								<div>
									<p class="font-medium">
										{bytes(
											snapshot.payload.memory.totalBytes - snapshot.payload.memory.availableBytes
										)}
									</p>
									<p class="text-muted-foreground">RAM used</p>
								</div>
								<div>
									<p class="font-medium">{bytes(snapshot.payload.memory.availableBytes)}</p>
									<p class="text-muted-foreground">RAM free</p>
								</div>
								<div>
									<p class="font-medium">{snapshot.payload.load1.toFixed(2)}</p>
									<p class="text-muted-foreground">Load 1m</p>
								</div>
							</div>
							<div class="rounded-md bg-muted/50 p-3 text-sm">
								<div class="flex justify-between">
									<span>{snapshot.payload.storage.path}</span><span
										>{bytes(snapshot.payload.storage.usedBytes)} / {bytes(
											snapshot.payload.storage.totalBytes
										)}</span
									>
								</div>
								<div class="mt-1 h-1.5 overflow-hidden rounded-full bg-muted">
									<div
										class="h-full bg-foreground/70"
										style={`width: ${ratio(snapshot.payload.storage.usedBytes, snapshot.payload.storage.totalBytes)}`}
									></div>
								</div>
							</div>
							{#if snapshot.payload.modelVersion}
								<p class="text-xs text-muted-foreground">
									UMAP model {snapshot.payload.modelVersion} · synced {ago(
										snapshot.payload.modelUpdatedAt ?? undefined
									)}
								</p>
							{/if}
							<div class="grid gap-2 text-sm">
								{#each snapshot.payload.containers as container (container.Name)}
									<div class="flex justify-between gap-3">
										<span class="truncate">{container.Name}</span><span
											class="shrink-0 text-muted-foreground"
											>{container.MemUsage} · {container.CPUPerc}</span
										>
									</div>
								{/each}
							</div>
						</Card.Content>
					{/if}
				</Card.Root>
			{/each}
		</section>

		<Card.Root>
			<Card.Header>
				<Card.Title>Largest database tables</Card.Title>
				<Card.Description>Total relation size, including indexes and TOAST data.</Card.Description>
			</Card.Header>
			<Card.Content class="grid gap-2 text-sm">
				{#each overview.database.largestTables as table (table.name)}
					<div class="flex justify-between">
						<span>{table.name}</span><span class="text-muted-foreground tabular-nums"
							>{bytes(table.bytes)}</span
						>
					</div>
				{/each}
			</Card.Content>
		</Card.Root>
	{/if}
</div>
