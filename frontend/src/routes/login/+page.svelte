<script lang="ts">
	import { resolve } from '$app/paths';
	import LogoMerged from '$lib/assets/logo_merged.svelte';
	import LoginForm from '$lib/components/login-form.svelte';
	import GalleryVerticalEndIcon from '@lucide/svelte/icons/gallery-vertical-end';
</script>

<div class="flex min-h-svh flex-col items-center justify-center gap-6 bg-muted p-6 md:p-10">
	<div class="flex w-full max-w-sm flex-col gap-6">
		<a href={resolve('/')} class="flex h-5 items-center gap-2 self-center font-medium">
			<LogoMerged />
		</a>
		<LoginForm />
	</div>
</div>
