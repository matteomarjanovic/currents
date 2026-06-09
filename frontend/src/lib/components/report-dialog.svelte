<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Label } from '$lib/components/ui/label';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import type { SaveView } from '$lib/types';

	interface Props {
		open: boolean;
		save: SaveView;
	}

	let { open = $bindable(), save }: Props = $props();

	const REASON_OPTIONS = [
		{ key: 'sexual', label: 'Sexual content', wireReasonType: 'sexual' },
		{ key: 'violence', label: 'Violent or graphic content', wireReasonType: 'violence' },
		{ key: 'ai-generated', label: 'Flag as AI-generated', wireReasonType: 'ai-generated' },
		{ key: 'other', label: 'Other', wireReasonType: 'other' }
	] as const;

	let reasonKey = $state<string>(REASON_OPTIONS[0].key);
	let reason = $state('');
	let submitting = $state(false);

	$effect(() => {
		if (!open) {
			reasonKey = REASON_OPTIONS[0].key;
			reason = '';
		}
	});

	async function submit(e: Event) {
		e.preventDefault();
		if (!auth.user) {
			open = false;
			promptLogin();
			return;
		}
		const opt = REASON_OPTIONS.find((o) => o.key === reasonKey) ?? REASON_OPTIONS[0];
		const trimmed = reason.trim() || undefined;

		submitting = true;
		try {
			const body = {
				reasonType: opt.wireReasonType,
				reason: trimmed,
				subject: {
					$type: 'com.atproto.repo.strongRef',
					uri: save.uri,
					// CID isn't available in SaveView; backend accepts empty cid
					// and treats labels as version-independent.
					cid: ''
				}
			};
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/xrpc/com.atproto.moderation.createReport`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body)
			});
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					open = false;
					promptLogin();
					return;
				}
				const text = await res.text();
				toast.error(`Report failed: ${text || res.status}`);
				return;
			}
			toast.success('Report submitted. Thank you.');
			open = false;
		} catch (err) {
			toast.error(String(err));
		} finally {
			submitting = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Report this image</Dialog.Title>
			<Dialog.Description>
				Your report goes to the Currents moderation team for review. False reports may be ignored.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={submit} class="flex flex-col gap-3">
			<fieldset class="flex flex-col gap-2" disabled={submitting}>
				<legend class="text-sm font-medium">Reason</legend>
				{#each REASON_OPTIONS as { key, label } (key)}
					<label
						class="flex items-center gap-2 rounded-md border border-border p-2 text-sm hover:bg-muted/40"
					>
						<input
							type="radio"
							name="report-reason-key"
							value={key}
							checked={reasonKey === key}
							onchange={() => (reasonKey = key)}
						/>
						<span>{label}</span>
					</label>
				{/each}
			</fieldset>
			<div class="flex flex-col gap-1">
				<Label for="report-reason" class="text-xs text-muted-foreground">
					Additional context (optional)
				</Label>
				<Textarea
					id="report-reason"
					rows={3}
					maxlength={2000}
					bind:value={reason}
					disabled={submitting}
				/>
			</div>
			<Dialog.Footer>
				<Button
					type="button"
					variant="outline"
					onclick={() => (open = false)}
					disabled={submitting}
				>
					Cancel
				</Button>
				<Button type="submit" disabled={submitting}>
					{submitting ? 'Submitting…' : 'Submit report'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
