<script lang="ts">
	import { apiFetch } from '$lib/api';
	import { onDestroy, onMount, tick } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Progress } from '$lib/components/ui/progress';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { RadioGroup } from 'bits-ui';
	import Check from '@lucide/svelte/icons/check';
	import {
		importBoardNotes,
		importCount,
		summarizeImport,
		type ImportJob
	} from '$lib/pinterest-import';
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

	interface SessionStatus {
		sessionId: string;
		jobs: ImportJob[];
	}

	interface ImportSession {
		sessionId: string;
		username: string;
		startedAt: string;
		active?: boolean;
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
	let retrying = $state(false);
	let history = $state<ImportSession[]>([]);
	let notice = $state('');
	let statusError = $state(false);
	let unavailableDestinations = $state(false);
	let showBoardDetails = $state(false);
	let historyDetails = $state<HTMLDetailsElement>();
	let historyTrigger = $state<HTMLElement>();
	let restored = false;

	$effect(() => {
		if (restored) return;
		if (!auth.user?.did) return;
		restored = true;
		void (async () => {
			try {
				const res = await apiFetch('/api/import/sessions');
				if (!res.ok) return;
				const data = (await res.json()) as { sessions: ImportSession[] };
				history = data.sessions;
				if (history[0]) showSession(history[0]);
			} catch {
				error = "Couldn't load previous imports. Please reload to try again.";
			}
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
				const message = res.headers.get('Content-Type')?.startsWith('text/plain')
					? (await res.text()).trim()
					: '';
				error = message || "Couldn't load boards right now. Please try again in a few minutes.";
				return;
			}
			const data = (await res.json()) as { boards: Board[] | null; username?: string };
			username = data.username ?? u;
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
		const res = await apiFetch(`/api/collection`, {
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
						pinterestPinCount: board.pinCount,
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
					pinterestPinCount: board.pinCount,
					pinterestUsername: uname,
					collectionUri: rootUri
				});
			}
			session = {
				sessionId,
				username: uname,
				startedAt: new Date().toISOString()
			};
			history = [session, ...history];
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
		const sessionId = session.sessionId;
		try {
			const res = await apiFetch(`/api/import/sessions/${sessionId}`);
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					promptLogin();
				}
				if (session?.sessionId === sessionId) statusError = true;
				return;
			}
			const next = (await res.json()) as SessionStatus;
			if (session?.sessionId === sessionId) {
				status = next;
				statusError = false;
			}
		} catch {
			if (session?.sessionId === sessionId) statusError = true;
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

	function showSession(next: ImportSession) {
		session = next;
		status = null;
		statusError = false;
		showBoardDetails = false;
		unavailableDestinations = false;
		error = null;
		notice = '';
		stage = 'progress';
		void refresh();
	}

	async function selectHistory(value: string) {
		const next = history.find((item) => item.sessionId === value);
		if (!next) return;
		showSession(next);
		if (historyDetails) historyDetails.open = false;
		await tick();
		historyTrigger?.focus();
	}

	async function retryMissing() {
		if (!session || retrying) return;
		retrying = true;
		unavailableDestinations = false;
		error = null;
		notice = '';
		const sessionId = session.sessionId;
		try {
			const res = await apiFetch(`/api/import/sessions/${sessionId}/retry`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({})
			});
			if (!res.ok) {
				if (res.status === 401) {
					promptLogin();
					return;
				}
				throw new Error('Could not retry this import. Please try again.');
			}
			const result = (await res.json()) as { retried: number; skipped: number };
			if (session?.sessionId !== sessionId) return;
			unavailableDestinations = result.retried === 0 && result.skipped > 0;
			notice =
				result.skipped > 0 && !unavailableDestinations
					? result.skipped === 1
						? '1 collection was removed and skipped.'
						: `${result.skipped} collections were removed and skipped.`
					: result.retried > 0
						? ''
						: unavailableDestinations
							? ''
							: 'There are no images to retry in this import.';
			await refresh();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Could not retry this import.';
		} finally {
			retrying = false;
		}
	}

	let summary = $derived.by(() => {
		const result = summarizeImport(status?.jobs ?? null, statusError);
		return unavailableDestinations && result.finished
			? {
					...result,
					state: 'removed',
					description: 'The collections that need retrying were removed.',
					hint: '',
					action: '' as const
				}
			: result;
	});
	let formattedStartedAt = $derived(session ? formatImportDate(session.startedAt) : '');

	function formatImportDate(value: string) {
		return new Date(value).toLocaleString(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		});
	}

	function runImportAction() {
		if (summary.action === 'reconnect') promptLogin();
		else if (summary.action === 'refresh') void refresh();
		else void retryMissing();
	}

	function generateUUID(): string {
		const b = crypto.getRandomValues(new Uint8Array(16));
		b[6] = (b[6] & 0x0f) | 0x40;
		b[8] = (b[8] & 0x3f) | 0x80;
		return [...b]
			.map((v, i) => ([4, 6, 8, 10].includes(i) ? '-' : '') + v.toString(16).padStart(2, '0'))
			.join('');
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
				<Label for="pinterest-username">Pinterest username or URL</Label>
				<Input
					id="pinterest-username"
					bind:value={username}
					placeholder="e.g. designspiration or a board URL"
					disabled={loading}
					autocorrect="off"
					autocapitalize="off"
					autocomplete="off"
					spellcheck={false}
					required
				/>
				<p class="text-sm text-muted-foreground">
					Import your public Pinterest boards. Your imported collections will be public.
				</p>
			</div>
			{#if error}
				<p class="text-sm text-destructive">{error}</p>
			{/if}
			<Button type="submit" disabled={loading}>
				{loading ? 'Loading…' : 'Load boards'}
			</Button>
		</form>
	{:else if stage === 'pick'}
		{#if boards.length > 0}
			<div class="mb-4 flex items-center justify-between">
				<p class="text-sm text-muted-foreground">
					{selected.size} of {boards.length} selected
				</p>
				<div class="flex gap-2">
					<Button variant="ghost" size="sm" onclick={selectAll}>Select all</Button>
					<Button variant="ghost" size="sm" onclick={selectNone}>Deselect all</Button>
				</div>
			</div>
		{/if}
		{#if boards.length === 0}
			<p class="text-sm text-muted-foreground">
				No public boards found for @{username}. Check that your boards are public in Pinterest, then
				try again.
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
		<div class="mt-6 space-y-3">
			{#if boards.length > 0}<p class="text-xs text-muted-foreground">
					Imported collections will be public.
				</p>{/if}
			<div class="flex items-center justify-between gap-3">
				<Button variant="ghost" onclick={() => (stage = 'username')} disabled={loading}>Back</Button
				>
				{#if boards.length > 0}<Button
						onclick={startImport}
						disabled={loading || selected.size === 0}
					>
						{loading
							? 'Starting…'
							: `Import ${selected.size} ${selected.size === 1 ? 'board' : 'boards'}`}
					</Button>{/if}
			</div>
		</div>
	{:else if stage === 'progress' && session}
		<div data-import-panel>
			{#if summary.finished}
				<div class="mb-4 flex justify-end">
					<Button onclick={startAnother}>Start another import</Button>
				</div>
			{/if}
			<Card.Root data-import-result>
				<Card.Header class="pb-2">
					<Card.Description class="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-xs">
						<span class="font-medium text-foreground">@{session.username || 'Pinterest'}</span>
						<time datetime={session.startedAt} class="whitespace-nowrap">{formattedStartedAt}</time>
					</Card.Description>
				</Card.Header>
				<Card.Content class="space-y-5">
					<div class="space-y-2" aria-live="polite">
						<h2 class="text-2xl font-semibold tracking-tight">{summary.title}</h2>
						{#if summary.description}<p class="text-sm text-muted-foreground">
								{summary.description}
							</p>{/if}
					</div>
					{#if summary.progress !== null}<Progress
							value={summary.progress}
							aria-label="Images processed"
						/>{/if}
					{#if summary.action}
						<Button disabled={retrying} onclick={runImportAction}
							>{retrying ? 'Starting…' : summary.actionLabel}</Button
						>
					{/if}
					{#if summary.hint}<p class="text-xs text-muted-foreground">{summary.hint}</p>{/if}
					{#if notice}<p class="text-sm" role="status">{notice}</p>{/if}
					{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}
					{#if status && status.jobs.length > 0}
						<details bind:open={showBoardDetails} class="border-t pt-4">
							<summary class="cursor-pointer text-sm font-medium">View board details</summary>
							{#if showBoardDetails}
								<ul class="mt-3 divide-y">
									{#each status.jobs as job (job.jobId)}
										<li class="space-y-1 py-3 first:pt-0">
											<div class="flex items-start justify-between gap-4 text-sm">
												<span class="min-w-0 font-medium">{job.boardName || 'Untitled board'}</span>
												<span class="shrink-0 text-muted-foreground"
													>{importCount(job.done)} saved</span
												>
											</div>
											{#each importBoardNotes(job) as note (note)}<p
													class="text-xs text-muted-foreground"
												>
													{note}
												</p>{/each}
										</li>
									{/each}
								</ul>
								<p class="mt-3 text-xs leading-relaxed text-muted-foreground">
									Pins can repeat the same image. Retries skip images imported earlier, including
									those you later moved or removed.
								</p>
							{/if}
						</details>
					{/if}
				</Card.Content>
			</Card.Root>
		</div>
	{/if}

	{#if history.length > 1}
		<details bind:this={historyDetails} class="mt-6 rounded-lg border p-3" data-import-history>
			<summary
				bind:this={historyTrigger}
				id="import-history-title"
				class="cursor-pointer text-sm font-medium">Import history</summary
			>
			<RadioGroup.Root
				class="mt-3 grid gap-3"
				aria-labelledby="import-history-title"
				value={session?.sessionId ?? ''}
				onValueChange={(value) => void selectHistory(value)}
			>
				{#each history as previous (previous.sessionId)}
					<Field.Label
						for={`import-session-${previous.sessionId}`}
						class="cursor-pointer transition-colors hover:bg-muted/40 has-[:focus-visible]:ring-[3px] has-[:focus-visible]:ring-ring/50 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-input/30"
					>
						<Field.Field orientation="horizontal">
							<Field.Content class="min-w-0">
								<Field.Title class="break-all">@{previous.username}</Field.Title>
								<Field.Description class="text-xs"
									><time datetime={previous.startedAt}>{formatImportDate(previous.startedAt)}</time
									></Field.Description
								>
							</Field.Content>
							<RadioGroup.Item
								id={`import-session-${previous.sessionId}`}
								value={previous.sessionId}
								class="flex size-4 shrink-0 items-center justify-center self-center rounded-full border border-input outline-none"
							>
								{#snippet children({ checked })}
									{#if checked}<span class="size-2 rounded-full bg-primary"></span>{/if}
								{/snippet}
							</RadioGroup.Item>
						</Field.Field>
					</Field.Label>
				{/each}
			</RadioGroup.Root>
		</details>
	{:else if history.length === 1 && stage !== 'progress'}
		<div class="mt-6">
			<Button variant="ghost" onclick={() => showSession(history[0])}>View last import</Button>
		</div>
	{/if}
</div>
