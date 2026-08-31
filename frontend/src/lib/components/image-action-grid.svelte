<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { copyImage, copyLink, downloadImage, shareLink } from '$lib/save-actions';
	import type { SaveView } from '$lib/types';
	import Copy from '@lucide/svelte/icons/copy';
	import Download from '@lucide/svelte/icons/download';
	import LinkIcon from '@lucide/svelte/icons/link';
	import Share2 from '@lucide/svelte/icons/share-2';

	let { item, onAction }: { item: SaveView; onAction?: () => void } = $props();

	function run(action: (save: SaveView) => Promise<void>) {
		onAction?.();
		void action(item);
	}
</script>

<div class="grid grid-cols-2 gap-2">
	<Button variant="outline" class="h-14 flex-col gap-1 pt-1" onclick={() => run(downloadImage)}>
		<Download />
		Download
	</Button>
	<Button variant="outline" class="h-14 flex-col gap-1 pt-1" onclick={() => run(copyImage)}>
		<Copy />
		Copy image
	</Button>
	<Button variant="outline" class="h-14 flex-col gap-1 pt-1" onclick={() => run(shareLink)}>
		<Share2 />
		Share
	</Button>
	<Button variant="outline" class="h-14 flex-col gap-1 pt-1" onclick={() => run(copyLink)}>
		<LinkIcon />
		Copy link
	</Button>
</div>
