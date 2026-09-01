<script lang="ts">
	import { toggleMode } from 'mode-watcher';
	import { Button } from '$lib/components/ui/button';
	import Sun from '@lucide/svelte/icons/sun';
	import Moon from '@lucide/svelte/icons/moon';

	let { class: className = '' }: { class?: string } = $props();
</script>

<Button onclick={toggleMode} variant="ghost" size="icon" class={className}>
	<Sun class="scale-100 rotate-0 !transition-all dark:scale-0 dark:-rotate-90" />
	<Moon class="absolute scale-0 rotate-90 !transition-all dark:scale-100 dark:rotate-0" />
	<span class="sr-only">Toggle theme</span>
</Button>
