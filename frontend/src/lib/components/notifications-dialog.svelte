<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import Bell from '@lucide/svelte/icons/bell';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import {
		notifications,
		refreshNotifications,
		type AttestationItem
	} from '$lib/stores/notifications.svelte';

	interface Props {
		open: boolean;
	}
	let { open = $bindable() }: Props = $props();

	let busyId = $state<number | null>(null);
	let selectedItem = $state<AttestationItem | null>(null);

	$effect(() => {
		if (open) {
			selectedItem = null;
			void refreshNotifications();
		}
	});

	function categoryLabel(item: AttestationItem): string {
		const cat = item.category ?? '';
		if (item.source === 'label_applied') {
			const v = item.labelVal ?? '';
			if (v === 'porn') return 'Sexually explicit label applied';
			if (v === 'nudity') return 'Nudity label applied';
			if (v === 'sexual') return 'Sexual content label applied';
			if (v === 'graphic-media') return 'Violent content label applied';
			if (v === 'currents-ai-generated') return 'AI-generated label applied';
			if (cat === 'nsfw') return 'Adult content label applied';
			if (cat === 'violence') return 'Violent content label applied';
			if (cat === 'ai-generated') return 'AI-generated label applied';
			return 'Content label applied';
		}
		const qualifier = (item.score ?? 0) >= 0.8 ? 'Likely' : 'Possibly';
		if (cat === 'nsfw') return `${qualifier} adult content`;
		if (cat === 'violence') return `${qualifier} violent content`;
		if (cat === 'ai-generated') return `${qualifier} AI-generated`;
		return `${qualifier} flagged content`;
	}

	async function confirm(item: AttestationItem) {
		busyId = item.id;
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/me/attestations/${item.id}/confirm`, {
				method: 'POST',
				credentials: 'include'
			});
			if (!res.ok) {
				const text = await res.text();
				toast.error(`Confirm failed: ${text || res.status}`);
				return;
			}
			toast.success('Label confirmed');
			await refreshNotifications();
		} catch (err) {
			toast.error(String(err));
		} finally {
			busyId = null;
		}
	}

	async function ignore(item: AttestationItem) {
		busyId = item.id;
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/me/attestations/${item.id}/ignore`, {
				method: 'POST',
				credentials: 'include'
			});
			if (!res.ok) {
				const text = await res.text();
				toast.error(`Failed: ${text || res.status}`);
				return;
			}
			toast.success('Dismissed');
			await refreshNotifications();
		} catch (err) {
			toast.error(String(err));
		} finally {
			busyId = null;
		}
	}

	async function dispute(item: AttestationItem) {
		busyId = item.id;
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/me/attestations/${item.id}/dispute`, {
				method: 'POST',
				credentials: 'include'
			});
			if (!res.ok) {
				const text = await res.text();
				toast.error(`Dispute failed: ${text || res.status}`);
				return;
			}
			toast.success('Dispute recorded — a moderator will decide.');
			await refreshNotifications();
		} catch (err) {
			toast.error(String(err));
		} finally {
			busyId = null;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="my-5 min-h-5/6 content-start">
		{#if selectedItem}
			{@const s = selectedItem}
			<div class="flex flex-col gap-4">
				<button
					type="button"
					onclick={() => (selectedItem = null)}
					class="flex w-fit items-center gap-1 text-sm text-muted-foreground transition-colors hover:text-foreground"
				>
					<ArrowLeft class="size-4" />
					Back
				</button>
				{#if s.previewUrl}
					<div class="overflow-hidden rounded-lg bg-muted">
						<img src={s.previewUrl} alt="" class="w-full object-cover" style="max-height: 18rem" />
					</div>
				{:else}
					<div class="h-48 rounded-lg bg-muted"></div>
				{/if}
				<div class="flex flex-col gap-2">
					<div class="flex flex-wrap items-center gap-1.5 text-xs">
						<Badge variant="secondary">{categoryLabel(s)}</Badge>
						{#if s.disputed}
							<Badge variant="outline">disputed</Badge>
						{/if}
					</div>
					<p class="text-sm leading-snug text-muted-foreground">
						{#if s.source === 'label_applied'}
							A content label was applied to this save. Labeled saves aren't removed — they stay
							visible in the app and are only hidden for users who choose to filter that content.
						{:else}
							Our auto-classifier flagged this image. No label has been applied yet — you can
							confirm this flag or dismiss it. Even if labeled, the save stays visible in the app
							and is only hidden for users who choose to filter that content.
						{/if}
					</p>
				</div>
				<div class="flex flex-wrap gap-2">
					{#if s.source === 'label_applied'}
						<Button
							size="sm"
							onclick={async () => {
								await ignore(s);
								selectedItem = null;
							}}
							disabled={busyId !== null}
						>
							Acknowledge
						</Button>
						<Button
							size="sm"
							variant="outline"
							onclick={async () => {
								await dispute(s);
								selectedItem = null;
							}}
							disabled={busyId !== null || s.disputed}
						>
							{s.disputed ? 'Disputed' : 'Dispute'}
						</Button>
					{:else}
						<Button
							size="sm"
							onclick={async () => {
								await confirm(s);
								selectedItem = null;
							}}
							disabled={busyId !== null}
						>
							Confirm
						</Button>
						<Button
							size="sm"
							variant="outline"
							onclick={async () => {
								await ignore(s);
								selectedItem = null;
							}}
							disabled={busyId !== null}
						>
							Ignore
						</Button>
					{/if}
				</div>
			</div>
		{:else}
			<Dialog.Header>
				<Dialog.Title class="flex items-center gap-2">
					<Bell class="size-4" />
					Notifications
				</Dialog.Title>
				<Dialog.Description>
					Pending labels on your saves. Confirm suspected content or dispute applied labels.
				</Dialog.Description>
			</Dialog.Header>

			{#if notifications.loading && notifications.items.length === 0}
				<div class="py-6 text-center text-sm text-muted-foreground">Loading…</div>
			{:else if notifications.error}
				<div
					class="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
				>
					{notifications.error}
				</div>
			{:else if notifications.items.length === 0}
				<div class="py-8 text-center text-sm text-muted-foreground">
					All caught up — no pending labels.
				</div>
			{:else}
				<ul class="flex max-h-[60vh] flex-col gap-2 overflow-y-auto">
					{#each notifications.items as item (item.id)}
						<li
							class="flex shrink-0 items-stretch gap-3 overflow-hidden rounded-lg border border-border bg-card"
						>
							{#if item.previewUrl}
								<button
									type="button"
									onclick={() => (selectedItem = item)}
									class="relative flex w-28 flex-shrink-0 cursor-pointer overflow-hidden bg-muted"
								>
									<img
										src={item.previewUrl}
										alt=""
										loading="lazy"
										class="h-full w-full object-cover transition-opacity hover:opacity-90"
									/>
								</button>
							{:else}
								<div class="w-28 flex-shrink-0 bg-muted"></div>
							{/if}
							<div class="flex min-w-0 flex-1 flex-col gap-1 p-2">
								<div class="flex flex-wrap items-center gap-1.5 text-xs">
									<Badge variant="secondary">{categoryLabel(item)}</Badge>
									{#if item.disputed}
										<Badge variant="outline">disputed</Badge>
									{/if}
								</div>
								<p class="my-1 text-xs leading-snug text-muted-foreground">
									{#if item.source === 'label_applied'}
										A content label was applied to this save. Labeled saves aren't removed — they
										stay visible in the app and are only hidden for users who choose to filter that
										content.
									{:else}
										Our auto-classifier flagged this image. No label has been applied yet — you can
										confirm this flag or dismiss it. Even if labeled, the save stays visible and is
										only hidden for users who choose to filter that content.
									{/if}
								</p>
								<div class="mt-auto flex flex-wrap gap-1.5">
									{#if item.source === 'label_applied'}
										<Button size="sm" onclick={() => ignore(item)} disabled={busyId !== null}>
											Acknowledge
										</Button>
										<Button
											size="sm"
											variant="outline"
											onclick={() => dispute(item)}
											disabled={busyId !== null || item.disputed}
										>
											{item.disputed ? 'Disputed' : 'Dispute'}
										</Button>
									{:else}
										<Button size="sm" onclick={() => confirm(item)} disabled={busyId !== null}>
											Confirm
										</Button>
										<Button
											size="sm"
											variant="outline"
											onclick={() => ignore(item)}
											disabled={busyId !== null}
										>
											Ignore
										</Button>
									{/if}
								</div>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		{/if}
	</Dialog.Content>
</Dialog.Root>
