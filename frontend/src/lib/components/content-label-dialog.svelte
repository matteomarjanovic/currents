<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { apiFetch } from '$lib/api';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import type { SaveView } from '$lib/types';

	interface Props {
		open: boolean;
		save: SaveView;
		onSaved: (added: string[]) => void;
	}

	let { open = $bindable(), save, onSaved }: Props = $props();

	const LABEL_OPTIONS = [
		{ val: 'porn', label: 'Porn' },
		{ val: 'sexual', label: 'Sexual' },
		{ val: 'nudity', label: 'Nudity' },
		{ val: 'graphic-media', label: 'Graphic' },
		{ val: 'currents-ai-generated', label: 'AI-generated' }
	];

	// Vals already on the save (any source) — shown locked, since removal is
	// intentionally not supported here (add-only).
	let appliedVals = $derived(new Set((save.labels ?? []).map((l) => l.val)));
	let pendingAdds = $state<Set<string>>(new Set());
	let submitting = $state(false);

	$effect(() => {
		if (!open) pendingAdds = new Set();
	});

	function toggleAdd(val: string) {
		if (appliedVals.has(val)) return; // applied labels are locked (add-only)
		const next = new Set(pendingAdds);
		if (next.has(val)) next.delete(val);
		else next.add(val);
		pendingAdds = next;
	}

	async function submit(e: Event) {
		e.preventDefault();
		if (pendingAdds.size === 0) return;
		submitting = true;
		const added = [...pendingAdds];
		try {
			const rkey = save.uri.split('/').pop() ?? '';
			const res = await apiFetch(`/save/${rkey}/labels`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
				body: new URLSearchParams({ labels: added.join(',') })
			});
			if (!res.ok) {
				if (res.status === 401) {
					auth.user = null;
					open = false;
					promptLogin();
					return;
				}
				toast.error(`Failed to apply labels (${res.status}).`);
				return;
			}
			onSaved(added);
			toast.success('Labels applied');
			open = false;
		} catch {
			toast.error('Network error. Please try again.');
		} finally {
			submitting = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Content labels</Dialog.Title>
			<Dialog.Description>
				Add content warnings to this image. Labels apply to every copy of this image and can't be
				removed here.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={submit} class="flex flex-col gap-4">
			<div class="flex flex-wrap items-center gap-1.5 text-sm">
				{#each LABEL_OPTIONS as opt (opt.val)}
					{@const applied = appliedVals.has(opt.val)}
					{@const pending = pendingAdds.has(opt.val)}
					<button
						type="button"
						disabled={applied || submitting}
						onclick={() => toggleAdd(opt.val)}
						title={applied ? 'Already applied' : 'Add this label'}
						class="rounded-full border px-2.5 py-1 transition-colors {applied || pending
							? 'border-foreground bg-foreground text-background'
							: 'border-border text-muted-foreground hover:bg-muted'} {applied
							? 'cursor-default opacity-90'
							: ''}"
					>
						{opt.label}
					</button>
				{/each}
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
				<Button type="submit" disabled={submitting || pendingAdds.size === 0}>
					{submitting ? 'Saving…' : 'Apply labels'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
