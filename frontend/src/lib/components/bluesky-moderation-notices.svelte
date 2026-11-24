<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, appviewUrl } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { bunnyImageUrl } from '$lib/image-url';
	import { isNative } from '$lib/platform';
	import { effectiveVisibility, prefsPending } from '$lib/stores/moderation-prefs.svelte';
	import type { LabelView } from '$lib/types';

	interface Action {
		type: string;
		createdAt: string;
		labels?: string[];
		policies?: { displayName: string }[];
	}

	interface Notice {
		uri: string;
		notice: {
			src: string;
			isRead: boolean;
			updatedAt: string;
			enforcement: { state: string; labels?: string[] };
			latestAction?: Action;
			availableActions?: string[];
		};
		previewUrl?: string;
		previewLabels?: LabelView[];
	}

	interface InboxPage {
		status: string;
		items?: Notice[];
		cursor?: string;
	}

	let { consentOutcome }: { consentOutcome?: string } = $props();
	let notices = $state<Notice[]>([]);
	let status = $state('');
	let cursor = $state('');
	let loading = $state(false);
	let detailURI = $state('');
	let detailError = $state('');

	const endpoint = '/api/me/bluesky-moderation';
	const inboxURL = 'https://bsky.app/moderation/inbox';
	const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' });

	onMount(() => {
		const request = new AbortController();
		void load('', request.signal);
		return () => request.abort();
	});

	function merge(items: Notice[]) {
		const byURI = new Map(notices.map((item) => [item.uri, item]));
		for (const item of items) {
			const previous = byURI.get(item.uri);
			if (!previous || Date.parse(item.notice.updatedAt) >= Date.parse(previous.notice.updatedAt)) {
				byURI.set(item.uri, item);
			}
		}
		notices = [...byURI.values()];
	}

	async function fetchPage(params: URLSearchParams, signal?: AbortSignal): Promise<InboxPage> {
		try {
			const res = await apiFetch(endpoint + '?' + params, { signal });
			if (res.ok) return (await res.json()) as InboxPage;
		} catch {
			// The inbox remains available independently of Currents notifications.
		}
		return { status: 'unavailable' };
	}

	async function load(after: string, signal?: AbortSignal) {
		loading = true;
		const data = await fetchPage(new URLSearchParams(after ? { cursor: after } : {}), signal);
		if (signal?.aborted) return;
		status = data.status;
		loading = false;
		if (data.status !== 'available') return;
		merge(data.items ?? []);
		cursor = data.cursor ?? '';
	}

	async function showDetails(item: Notice) {
		if (detailURI === item.uri) {
			detailURI = '';
			return;
		}
		detailURI = item.uri;
		detailError = '';
		const data = await fetchPage(new URLSearchParams({ subject: item.uri }));
		if (detailURI !== item.uri) return;
		const detail = data.items?.find(({ uri }) => uri === item.uri);
		if (detail) merge([detail]);
		else detailError = 'Bluesky could not load more details.';
	}

	function title({ notice }: Notice) {
		switch (notice.latestAction?.type) {
			case 'labelApplied':
				return 'Content label applied';
			case 'labelRemoved':
				return 'Content label removed';
		}
		if (notice.enforcement.state === 'labeled') return 'Content labeled';
		if (['removed', 'takendown'].includes(notice.enforcement.state)) return 'Content removed';
		return 'Moderation decision';
	}

	function appealURL(item: Notice) {
		const params = new URLSearchParams({ moderationService: item.notice.src, subject: item.uri });
		return inboxURL + '/notice/details?' + params;
	}
</script>

<section aria-label="Bluesky moderation notices" class="flex flex-col gap-2 px-3 pb-3">
	{#if loading && notices.length === 0}
		<p class="text-sm text-muted-foreground">Checking Bluesky moderation…</p>
	{:else if status === 'connect'}
		{#if consentOutcome === 'denied'}
			<p class="text-sm text-muted-foreground">Bluesky inbox access was declined.</p>
		{:else if consentOutcome === 'incomplete'}
			<p class="text-sm text-muted-foreground">
				Some required permissions are missing. Review them when you reconnect.
			</p>
		{:else if consentOutcome === 'session-changed'}
			<p class="text-sm text-muted-foreground">
				Sign in to Currents with the same account and try again.
			</p>
		{:else if consentOutcome === 'unavailable'}
			<p class="text-sm text-muted-foreground">
				Currents couldn’t complete the connection. Try again.
			</p>
		{/if}
		{#if isNative()}
			<p class="text-sm text-muted-foreground">
				Open Currents in your browser to connect the Bluesky moderation inbox.
			</p>
		{:else}
			<form
				method="POST"
				action={appviewUrl('/oauth/bluesky-moderation')}
				class="flex flex-col items-start gap-2 px-1"
			>
				<p class="text-sm text-muted-foreground">
					See Bluesky moderation notices about your Currents saves.
				</p>
				<Button type="submit" size="sm" variant="outline">Connect Bluesky inbox</Button>
			</form>
		{/if}
	{:else if status === 'permission-denied' || status === 'unsupported'}
		<p class="text-sm text-muted-foreground">
			Bluesky inbox access is unavailable for this account.
		</p>
	{:else if status === 'unavailable'}
		<p class="text-sm text-muted-foreground">Bluesky moderation is temporarily unavailable.</p>
		<Button size="sm" variant="outline" class="self-start" onclick={() => load(cursor)}
			>Try again</Button
		>
	{:else if status === 'available' && notices.length === 0}
		<p class="text-sm text-muted-foreground">
			No Bluesky notices about Currents saves on this page.
		</p>
	{/if}

	{#each notices as item (item.uri)}
		{@const vis = effectiveVisibility(item.previewLabels)}
		{@const action = item.notice.latestAction}
		{@const labels = [
			...new Set([...(action?.labels ?? []), ...(item.notice.enforcement.labels ?? [])])
		]}
		{@const reasons = [...new Set(action?.policies?.map((policy) => policy.displayName))]}
		{@const date = new Date(action?.createdAt ?? item.notice.updatedAt)}
		<article class="flex gap-3 overflow-hidden rounded-lg border border-border bg-card p-2">
			{#if item.previewUrl && vis !== 'hide'}
				<div class="size-20 shrink-0 overflow-hidden rounded bg-muted">
					{#if !item.previewLabels?.length || !prefsPending()}
						<img
							src={bunnyImageUrl(item.previewUrl, { width: 256, quality: 75 })}
							alt="Currents save"
							loading="lazy"
							class={['size-full object-cover', { 'blur-xl': vis === 'blur' }]}
						/>
					{/if}
				</div>
			{/if}
			<div class="flex min-w-0 flex-col items-start gap-1">
				<div class="flex flex-wrap items-center gap-1.5">
					<Badge variant="outline">Bluesky</Badge>
					{#if !item.notice.isRead}<Badge variant="secondary">Unread in Bluesky</Badge>{/if}
				</div>
				<p class="text-sm font-medium">{title(item)}</p>
				{#if labels.length}<p class="text-xs text-muted-foreground">{labels.join(', ')}</p>{/if}
				{#each reasons as reason (reason)}
					<p class="text-xs text-muted-foreground">{reason}</p>
				{/each}
				{#if !Number.isNaN(date.getTime())}
					<time class="text-xs text-muted-foreground" datetime={date.toISOString()}
						>{dateFormat.format(date)}</time
					>
				{/if}
				<div class="flex flex-wrap items-center gap-2">
					<Button
						size="sm"
						variant="ghost"
						onclick={() => showDetails(item)}
						aria-expanded={detailURI === item.uri}
					>
						{detailURI === item.uri ? 'Close details' : 'Check decision'}
					</Button>
					{#if item.notice.availableActions?.includes('appeal')}
						<Button
							size="sm"
							variant="outline"
							href={appealURL(item)}
							target="_blank"
							rel="noopener noreferrer"
						>
							Appeal in Bluesky
						</Button>
					{/if}
				</div>
				{#if detailURI === item.uri}
					{#if detailError}<p class="text-xs text-muted-foreground">{detailError}</p>{/if}
					<a
						class="text-xs underline"
						href={appealURL(item)}
						target="_blank"
						rel="noopener noreferrer">Open notice in Bluesky</a
					>
				{/if}
			</div>
		</article>
	{/each}
	{#if cursor && status === 'available'}
		<Button
			size="sm"
			variant="outline"
			class="self-start"
			disabled={loading}
			onclick={() => load(cursor)}
		>
			Load older Bluesky notices
		</Button>
	{/if}
</section>
