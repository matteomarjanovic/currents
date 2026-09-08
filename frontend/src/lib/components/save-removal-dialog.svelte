<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { answerLastSaveRemoval, saveRemovalDialog } from '$lib/stores/save-removal-dialog.svelte';
	import {
		setLastSaveRemovalAction,
		type LastSaveRemovalAction
	} from '$lib/stores/preferences.svelte';

	let wasOpen = false;
	$effect(() => {
		const open = saveRemovalDialog.open;
		if (wasOpen && !open) answerLastSaveRemoval(null);
		wasOpen = open;
	});

	function choose(action: LastSaveRemovalAction) {
		if (saveRemovalDialog.remember) setLastSaveRemovalAction(action);
		answerLastSaveRemoval(action);
	}
</script>

<AlertDialog.Root bind:open={saveRemovalDialog.open}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>
				{saveRemovalDialog.count === 1
					? 'This image is only saved in this collection'
					: `${saveRemovalDialog.count} images are only saved in this collection`}
			</AlertDialog.Title>
			<AlertDialog.Description>
				{saveRemovalDialog.count === 1
					? 'What do you want to do with it?'
					: 'What do you want to do with them?'}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<label class="flex items-start gap-2 text-sm">
			<input
				type="checkbox"
				bind:checked={saveRemovalDialog.remember}
				class="mt-0.5 accent-foreground"
			/>
			<span>Use my choice as the default. You can change it in Settings.</span>
		</label>
		<AlertDialog.Footer class="sm:flex-wrap">
			<AlertDialog.Cancel onclick={() => answerLastSaveRemoval(null)}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="outline" onclick={() => choose('move-to-profile')}>
				Move to Profile
			</AlertDialog.Action>
			<AlertDialog.Action variant="destructive" onclick={() => choose('delete')}>
				Delete permanently
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
