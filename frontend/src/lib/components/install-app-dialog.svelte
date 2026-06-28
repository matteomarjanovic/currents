<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { isIosWeb } from '$lib/platform';

	interface Props {
		open: boolean;
	}

	let { open = $bindable() }: Props = $props();

	// Shown only when the browser can't trigger install programmatically (iOS Safari always;
	// other browsers when no beforeinstallprompt is available). Pick instructions per platform.
	const ios = isIosWeb();
</script>

<Dialog.Root bind:open>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Install Currents</Dialog.Title>
			<Dialog.Description>
				Add Currents to your home screen for a full-screen, app-like experience.
			</Dialog.Description>
		</Dialog.Header>
		<ol class="list-decimal space-y-2 pl-5 text-sm text-muted-foreground">
			{#if ios}
				<li>
					Tap the <span class="font-medium text-foreground">Share</span> button in the toolbar.
				</li>
				<li>Choose <span class="font-medium text-foreground">Add to Home Screen</span>.</li>
				<li>Tap <span class="font-medium text-foreground">Add</span>.</li>
			{:else}
				<li>Open your browser's menu.</li>
				<li>
					Choose <span class="font-medium text-foreground">Install app</span> or
					<span class="font-medium text-foreground">Add to Home screen</span>.
				</li>
			{/if}
		</ol>
		<Dialog.Footer>
			<Button variant="ghost" onclick={() => (open = false)}>Got it</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
