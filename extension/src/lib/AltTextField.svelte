<script lang="ts">
	// import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';

	interface Props {
		imageUrl: string;
		suggestion?: string;
		value: string;
		disabled?: boolean;
		oninput?: () => void;
	}
	let {
		imageUrl,
		suggestion = '',
		value = $bindable(''),
		disabled = false,
		oninput
	}: Props = $props();
	const id = $props.id();
	/* Suggestions are paused; keep the implementation for a future revisit.
	let networkSuggestion = $state('');
	let textarea = $state<HTMLTextAreaElement | null>(null);
	let proposed = $derived((suggestion.trim() || networkSuggestion).slice(0, 2000));

	// Suggestions stay separate from the value that is saved, including when a
	// network lookup finishes after the user has started writing.
	$effect(() => {
		const url = imageUrl;
		networkSuggestion = '';
		if (suggestion.trim()) return;
		let active = true;
		void browser.runtime
			.sendMessage({ type: 'LOOKUP_ALT', imgUrl: url })
			.then((response) => {
				if (active) networkSuggestion = response?.alt?.trim() ?? '';
			})
			.catch(() => {});
		return () => {
			active = false;
		};
	});
	*/
</script>

<div class="flex flex-col gap-2">
	<label for={id} class="text-xs text-muted-foreground">Alt text</label>
	<Textarea
		{id}
		placeholder="Describe the image for people who use screen readers (optional)"
		bind:value
		{oninput}
		{disabled}
		maxlength={2000}
		rows={2}
	/>
	<!-- Suggestions are paused. Restore bind:ref={textarea} above when enabling.
	{#if proposed && !value.trim()}
		<div class="flex flex-col gap-1">
			<div class="flex items-center justify-between gap-2">
				<p class="text-xs text-muted-foreground">
					{suggestion.trim() ? 'Suggested from this page' : 'Suggested from a previous save'}
				</p>
				<Button
					variant="link"
					size="xs"
					class="h-auto shrink-0 p-0 text-xs"
					{disabled}
					onclick={() => {
						value = proposed;
						textarea?.focus();
					}}>Use suggestion</Button
				>
			</div>
			<p class="max-h-16 overflow-y-auto text-xs break-words whitespace-pre-wrap">{proposed}</p>
		</div>
	{/if}
	-->
</div>
