<script lang="ts">
	import SaveDetail from '$lib/components/save-detail.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
</script>

<SaveDetail save={data.save} />
