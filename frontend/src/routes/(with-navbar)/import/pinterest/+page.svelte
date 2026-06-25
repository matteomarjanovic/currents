<script lang="ts">
	import { apiFetch } from '$lib/api';
	import { onDestroy, onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Progress } from '$lib/components/ui/progress';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import Check from '@lucide/svelte/icons/check';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { markFeatureSeen, FEATURE_PINTEREST_IMPORT } from '$lib/stores/features.svelte';

	type Stage = 'username' | 'pick' | 'progress';

	interface Board {
		id: string;
		name: string;
		pinCount: number;
		url: string;
	}

	interface Section {
		id: string;
		title: string;
		url: string;
	}

	type JobPhase = 'listing' | 'running' | 'done' | 'failed';

	interface JobStatus {
		jobId: string;
		boardName: string;
		status: JobPhase;
		queued: number;
		running: number;
		done: number;
		failed: number;
	}

	interface SessionStatus {
		sessionId: string;
		jobs: JobStatus[];
	}

	interface ImportSession {
		sessionId: string;
		username: string;
		startedAt: string;
	}

	let stage = $state<Stage>('username');
	let username = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);

	let boards = $state<Board[]>([]);
	let selected = new SvelteSet<string>();

	let session = $state<ImportSession | null>(null);
	let status = $state<SessionStatus | null>(null);
	let pollTimer: ReturnType<typeof setInterval> | null = null;
	let polling = false;
	let restored = false;

	$effect(() => {
		if (restored) return;
		if (!auth.user?.did) return;
		restored = true;
		void (async () => {
			const res = await apiFetch(`/api/import/active-session`);
			if (!res.ok) return;
			const data = (await res.json()) as {
				sessionId?: string;
				username?: string;
				startedAt?: string;
			};
			if (!data.sessionId) return;
			session = {
				sessionId: data.sessionId,
				username: data.username ?? '',
				startedAt: data.startedAt ?? new Date().toISOString()
			};
			stage = 'progress';
			void refresh();
		})();
	});

	$effect(() => {
		if (stage !== 'progress' || !session) {
			stopPolling();
			return;
		}
		const anyRunning =
			!status || status.jobs.some((j) => j.status === 'listing' || j.status === 'running');
		if (anyRunning && !pollTimer) {
			pollTimer = setInterval(() => void refresh(), 3000);
		} else if (!anyRunning && pollTimer) {
			stopPolling();
		}
	});

	onMount(() => markFeatureSeen(FEATURE_PINTEREST_IMPORT));
	onDestroy(() => stopPolling());

	function stopPolling() {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	async function loadBoards(e: Event) {
		e.preventDefault();
		const u = username.trim();
		if (!u) {
			error = 'Username is required.';
			return;
		}
		loading = true;
		error = null;
		try {
			const res = await apiFetch(`/api/import/pinterest/boards?username=${encodeURIComponent(u)}`);
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
					return;
				}
				error = `Couldn't load boards (${res.status}).`;
				return;
			}
			const data = (await res.json()) as { boards: Board[] | null };
			boards = data.boards ?? [];
			selected.clear();
			for (const b of boards) selected.add(b.id);
			stage = 'pick';
		} catch {
			error = 'Network error. Please try again.';
		} finally {
			loading = false;
		}
	}

	function toggle(id: string) {
		if (selected.has(id)) selected.delete(id);
		else selected.add(id);
	}

	function selectAll() {
		for (const b of boards) selected.add(b.id);
	}

	function selectNone() {
		selected.clear();
	}

	class Unauthorized extends Error {}

	async function createCollection(
		name: string,
		description: string,
		parent?: string
	): Promise<string> {
		const res = await apiFetch(`/collection`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'application/json' },
			body: new URLSearchParams({ name, description, ...(parent ? { parent } : {}) }).toString()
		});
		if (!res.ok) {
			if (res.status === 401) throw new Unauthorized();
			throw new Error(`creating collection ${name} (${res.status})`);
		}
		const { uri } = (await res.json()) as { uri: string };
		return uri;
	}

	async function queueJob(payload: Record<string, unknown>) {
		const res = await apiFetch(`/api/import/pinterest/jobs`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
			body: JSON.stringify(payload)
		});
		if (!res.ok) {
			if (res.status === 401) throw new Unauthorized();
			throw new Error(`queueing job (${res.status})`);
		}
	}

	async function fetchSections(board: Board): Promise<Section[]> {
		const res = await apiFetch(
			`/api/import/pinterest/sections?boardId=${encodeURIComponent(board.id)}&boardUrl=${encodeURIComponent(board.url)}`
		);
		if (!res.ok) {
			if (res.status === 401) throw new Unauthorized();
			// Sections are best-effort; fall back to a flat board import.
			return [];
		}
		const data = (await res.json()) as { sections: Section[] | null };
		return data.sections ?? [];
	}

	async function startImport() {
		const did = auth.user?.did;
		if (!did) {
			promptLogin();
			return;
		}
		if (selected.size === 0) return;
		loading = true;
		error = null;
		try {
			const sessionId = crypto.randomUUID?.() ?? generateUUID();
			const uname = username.trim();
			const picked = boards.filter((b) => selected.has(b.id));
			for (const board of picked) {
				const boardLink = `https://www.pinterest.com${board.url}`;
				const rootUri = await createCollection(
					board.name,
					`Imported from Pinterest board "${board.name}" by @${uname} (${boardLink})`
				);
				const sections = await fetchSections(board);
				for (const section of sections) {
					const sectionLink = `https://www.pinterest.com${section.url}`;
					const subUri = await createCollection(
						section.title,
						`Imported from Pinterest section "${section.title}" in board "${board.name}" by @${uname} (${sectionLink})`,
						rootUri
					);
					await queueJob({
						importSessionId: sessionId,
						pinterestBoardId: board.id,
						pinterestBoardName: `${board.name} › ${section.title}`,
						pinterestBoardUrl: section.url,
						pinterestSectionId: section.id,
						pinterestUsername: uname,
						collectionUri: subUri
					});
				}
				await queueJob({
					importSessionId: sessionId,
					pinterestBoardId: board.id,
					pinterestBoardName: board.name,
					pinterestBoardUrl: board.url,
					filterSectionPins: sections.length > 0,
					pinterestUsername: uname,
					collectionUri: rootUri
				});
			}
			session = {
				sessionId,
				username: uname,
				startedAt: new Date().toISOString()
			};
			status = null;
			stage = 'progress';
			void refresh();
		} catch (e) {
			if (e instanceof Unauthorized) {
				auth.user = null;
				promptLogin();
				return;
			}
			error = e instanceof Error ? e.message : 'Failed to start import.';
		} finally {
			loading = false;
		}
	}

	async function refresh() {
		if (!session || polling) return;
		polling = true;
		try {
			const res = await apiFetch(`/api/import/sessions/${session.sessionId}`);
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
				}
				return;
			}
			status = (await res.json()) as SessionStatus;
		} catch {
			// network blip — next tick will retry
		} finally {
			polling = false;
		}
	}

	function startAnother() {
		session = null;
		status = null;
		boards = [];
		selected.clear();
		username = '';
		error = null;
		stage = 'username';
	}

	let totalProcessed = $derived(
		status ? status.jobs.reduce((s, j) => s + j.done + j.failed, 0) : 0
	);
	let totalItems = $derived(
		status ? status.jobs.reduce((s, j) => s + j.queued + j.running + j.done + j.failed, 0) : 0
	);
	let progressValue = $derived(totalItems === 0 ? 0 : (totalProcessed / totalItems) * 100);
	let allDone = $derived(
		status !== null &&
			status.jobs.length > 0 &&
			status.jobs.every((j) => j.status === 'done' || j.status === 'failed')
	);
	let formattedStartedAt = $derived(session ? new Date(session.startedAt).toLocaleString() : '');

	function generateUUID(): string {
		const b = crypto.getRandomValues(new Uint8Array(16));
		b[6] = (b[6] & 0x0f) | 0x40;
		b[8] = (b[8] & 0x3f) | 0x80;
		return [...b]
			.map((v, i) => ([4, 6, 8, 10].includes(i) ? '-' : '') + v.toString(16).padStart(2, '0'))
			.join('');
	}

	function badgeVariant(s: JobPhase): 'default' | 'secondary' | 'destructive' | 'outline' {
		if (s === 'failed') return 'destructive';
		if (s === 'done') return 'secondary';
		return 'outline';
	}
</script>

<svelte:head>
	<title>Import from Pinterest · Currents</title>
</svelte:head>

<div class="mx-auto max-w-2xl">
	<h1 class="mb-6 text-2xl font-semibold">Import from Pinterest</h1>

	{#if stage === 'username'}
		<form onsubmit={loadBoards} class="space-y-4">
			<div class="space-y-2">
				<Label for="pinterest-username">Pinterest username</Label>
				<Input
					id="pinterest-username"
					bind:value={username}
					placeholder="e.g. designspiration"
					disabled={loading}
					autocorrect="off"
					autocapitalize="off"
					autocomplete="off"
					spellcheck={false}
					required
				/>
				<div class="mt-2 text-sm text-muted-foreground">
					IMPORTANT:
					<ul class="ml-4 list-disc">
						<li>Only public boards are importable</li>
						<li>The boards on Currents are only public (for now)</li>
					</ul>
				</div>
			</div>
			{#if error}
				<p class="text-sm text-destructive">{error}</p>
			{/if}
			<Button type="submit" disabled={loading}>
				{loading ? 'Loading…' : 'Load boards'}
			</Button>
		</form>
	{:else if stage === 'pick'}
		<div class="mb-4 flex items-center justify-between">
			<p class="text-sm text-muted-foreground">
				{selected.size} of {boards.length} selected
			</p>
			<div class="flex gap-2">
				<Button variant="ghost" size="sm" onclick={selectAll}>Select all</Button>
				<Button variant="ghost" size="sm" onclick={selectNone}>Deselect all</Button>
			</div>
		</div>
		{#if boards.length === 0}
			<p class="text-sm text-muted-foreground">
				No public boards found for @{username}.
			</p>
		{:else}
			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
				{#each boards as board (board.id)}
					{@const isSel = selected.has(board.id)}
					<button
						type="button"
						onclick={() => toggle(board.id)}
						data-selected={isSel}
						class="group data-[selected=true]:ring-0.5 relative flex flex-col items-start gap-1 rounded-xl border bg-card p-4 text-left transition-colors hover:bg-accent/40 data-[selected=true]:bg-accent"
					>
						<div class="pr-7 font-medium">{board.name}</div>
						<div class="text-xs text-muted-foreground">
							{board.pinCount}
							{board.pinCount === 1 ? 'pin' : 'pins'}
						</div>
						{#if isSel}
							<div
								class="absolute top-3 right-3 flex size-5 items-center justify-center rounded-full bg-primary text-primary-foreground"
							>
								<Check class="size-3" />
							</div>
						{/if}
					</button>
				{/each}
			</div>
		{/if}
		{#if error}
			<p class="mt-4 text-sm text-destructive">{error}</p>
		{/if}
		<div class="mt-6 flex items-center justify-between">
			<Button variant="ghost" onclick={() => (stage = 'username')} disabled={loading}>Back</Button>
			<div class="flex items-center justify-center gap-2">
				<p class="text-sm text-muted-foreground">The boards in Currents will be public.</p>
				<Button onclick={startImport} disabled={loading || selected.size === 0}>
					{loading
						? 'Starting…'
						: `Import ${selected.size} ${selected.size === 1 ? 'board' : 'boards'}`}
				</Button>
			</div>
		</div>
	{:else if stage === 'progress' && session}
		<Card.Root>
			<Card.Header>
				<Card.Title>{session.username ? `@${session.username}` : 'Pinterest import'}</Card.Title>
				<Card.Description>Started {formattedStartedAt}</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-4">
				<div class="space-y-1.5">
					<Progress value={progressValue} />
					<div class="flex items-center justify-between text-xs text-muted-foreground">
						<span>{totalProcessed} of {totalItems} pins</span>
						{#if allDone}
							<span class="flex items-center gap-1 text-foreground">
								<Check class="size-3.5" /> Done
							</span>
						{/if}
					</div>
				</div>
				{#if !allDone}
					<p class="text-xs leading-relaxed text-muted-foreground">
						Large imports can take a while — hours, or longer for big boards. If progress looks
						stuck, your data server is briefly limiting uploads; the import keeps going on its own
						and resumes automatically. You can safely leave this page — it continues in the
						background.
					</p>
				{/if}
				{#if status && status.jobs.length > 0}
					<ul class="space-y-2">
						{#each status.jobs as job (job.jobId)}
							{@const total = job.queued + job.running + job.done + job.failed}
							{@const processed = job.done + job.failed}
							<li class="flex items-center justify-between gap-3 rounded-md border p-3">
								<div class="min-w-0 flex-1">
									<div class="truncate text-sm font-medium">{job.boardName || '—'}</div>
									<div class="text-xs text-muted-foreground">
										{processed} / {total} pins{#if job.failed > 0}
											· {job.failed} failed{/if}
									</div>
								</div>
								<Badge variant={badgeVariant(job.status)}>{job.status}</Badge>
							</li>
						{/each}
					</ul>
				{:else if !status}
					<p class="text-sm text-muted-foreground">Loading status…</p>
				{/if}
			</Card.Content>
		</Card.Root>
		<div class="mt-6">
			<Button onclick={startAnother} variant="outline" disabled={!allDone}>
				{allDone ? 'Start another import' : 'Import in progress, you can leave this page'}
			</Button>
		</div>
	{/if}
</div>
