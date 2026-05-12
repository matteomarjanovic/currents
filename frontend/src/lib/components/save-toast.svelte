<script lang="ts">
	interface Props {
		imageUrl?: string;
		collectionName: string;
	}

	let { imageUrl, collectionName }: Props = $props();
</script>

<div class="flex items-center gap-3">
	{#if imageUrl}
		<img
			src={imageUrl}
			alt=""
			class="size-10 shrink-0 rounded-sm bg-muted object-cover"
		/>
	{:else}
		<div class="size-10 shrink-0 rounded-sm bg-muted"></div>
	{/if}
	<div class="min-w-0 text-sm">
		<span class="text-muted-foreground">Saved to </span>
		<span class="font-medium text-foreground">{collectionName}</span>
	</div>
</div>
