<script lang="ts">
	import { goto } from '$app/navigation';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { loginPrompt } from '$lib/stores/login-prompt.svelte';
	import LogoMerged from '$lib/assets/logo.svelte';
</script>

<Dialog.Root bind:open={loginPrompt.open}>
	<Dialog.Content>
		<div class="flex justify-center">
			<div class="h-5">
				<LogoMerged />
			</div>
		</div>
		<Dialog.Header>
			<Dialog.Title>Log in to continue</Dialog.Title>
			<Dialog.Description>
				Sign in with your Bluesky account to save images and build collections.
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button
				onclick={() => {
					loginPrompt.open = false;
					goto('/login');
				}}
			>
				Log in
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
