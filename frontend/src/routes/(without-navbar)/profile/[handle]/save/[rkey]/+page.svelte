<script lang="ts">
	import SaveDetail from '$lib/components/save-detail.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const author = $derived(data.save.author.displayName || '@' + data.save.author.handle);
	const title = $derived((data.save.text?.trim() || 'Save by ' + author).slice(0, 70));
</script>

<svelte:head>
	<title>{title + ' · Currents'}</title>
</svelte:head>

<SaveDetail save={data.save} />
