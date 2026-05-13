<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';

	interface Props {
		open: boolean;
	}

	let { open = $bindable() }: Props = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Browser extension</Dialog.Title>
			<Dialog.Description>
				The Currents extension isn't available for Safari yet. You can install it on a Chrome- or
				Firefox-based browser instead.
			</Dialog.Description>
		</Dialog.Header>
		<div class="flex flex-col gap-2">
			<Button
				variant="outline"
				href="https://chromewebstore.google.com/detail/save-to-currents/kdifjldjjhopgdhppjpknloichglmmdi"
				target="_blank"
				rel="noopener"
			>
				Chrome Web Store
			</Button>
			<Button
				variant="outline"
				href="https://addons.mozilla.org/en-US/firefox/addon/save-to-currents/"
				target="_blank"
				rel="noopener"
			>
				Firefox Add-ons
			</Button>
		</div>
		<Dialog.Footer>
			<Button variant="ghost" onclick={() => (open = false)}>Close</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
