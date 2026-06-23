<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { auth } from '$lib/stores/auth.svelte';

	let {
		open = $bindable(false),
		description = "To follow users, Currents needs permission to create follow records on your AT Protocol account. You'll be redirected to re-authorize — it only takes a moment."
	}: { open?: boolean; description?: string } = $props();

	function reauthorize() {
		const form = document.createElement('form');
		form.method = 'POST';
		form.action = `${PUBLIC_APPVIEW_URL}/oauth/login`;
		const u = document.createElement('input');
		u.name = 'username';
		u.value = auth.user?.handle ?? '';
		const r = document.createElement('input');
		r.name = 'return_to';
		r.value = window.location.href;
		form.append(u, r);
		document.body.append(form);
		form.submit();
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="max-w-sm">
		<Dialog.Header>
			<Dialog.Title>New permission required</Dialog.Title>
			<Dialog.Description>
				{description}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (open = false)}>Cancel</Button>
			<Button onclick={reauthorize}>Re-authorize</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
