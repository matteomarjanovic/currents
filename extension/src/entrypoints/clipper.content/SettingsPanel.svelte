<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Switch } from '$lib/components/ui/switch';
	import { scrollFade } from '../../lib/scroll-fade';
	import {
		isCurrentsHost,
		normalizeDomain,
		saveButtonSettings,
		showButtonOnSite,
		updateSaveButtonSettings
	} from '../../lib/save-button-settings.svelte';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import Plus from '@lucide/svelte/icons/plus';
	import X from '@lucide/svelte/icons/x';

	let { onBack }: { onBack: () => void } = $props();
	let domainInput = $state('');
	let error = $state('');
	const currentDomain = normalizeDomain(location.hostname);
	let currentAdded = $derived(currentDomain ? showButtonOnSite(currentDomain) : false);

	function addDomain(value: string) {
		const domain = normalizeDomain(value);
		if (!domain) {
			error = 'Enter a valid website domain or URL.';
			return;
		}
		if (isCurrentsHost(domain)) {
			error = 'The save button is always hidden on Currents.';
			return;
		}
		if (saveButtonSettings.domains.includes(domain)) {
			error = 'This website is already in the list.';
			return;
		}
		void updateSaveButtonSettings({
			allSites: saveButtonSettings.allSites,
			domains: [...saveButtonSettings.domains, domain]
		});
		domainInput = '';
		error = '';
	}

	function removeDomain(domain: string) {
		void updateSaveButtonSettings({
			allSites: saveButtonSettings.allSites,
			domains: saveButtonSettings.domains.filter((value) => value !== domain)
		});
		error = '';
	}

	function handleDomainKeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter' && event.key !== ',') return;
		event.preventDefault();
		event.stopPropagation();
		if (domainInput.trim()) addDomain(domainInput);
	}
</script>

<div class="flex shrink-0 items-center gap-3">
	<Button variant="outline" size="icon-sm" class="rounded-full" aria-label="Back" onclick={onBack}>
		<ChevronLeft class="size-4" />
	</Button>
	<h2 class="text-lg font-semibold">Settings</h2>
</div>

<div use:scrollFade class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pb-1">
	<section
		class="flex items-center justify-between gap-4 rounded-lg border border-border bg-card p-4"
	>
		<div class="flex flex-col gap-0.5">
			<span class="text-sm font-medium">Save button on every site</span>
			<span class="text-xs text-muted-foreground"> Show over images outside currents.is. </span>
		</div>
		<Switch
			aria-label="Save button on every site"
			checked={saveButtonSettings.allSites}
			disabled={!saveButtonSettings.ready}
			onCheckedChange={(allSites) =>
				void updateSaveButtonSettings({ allSites, domains: saveButtonSettings.domains })}
		/>
	</section>

	{#if saveButtonSettings.ready && !saveButtonSettings.allSites}
		<section class="rounded-2xl border border-border p-3">
			<label for="save-button-domain" class="font-medium">Selected websites</label>
			<p class="mt-1 text-xs text-muted-foreground">Show the button on these domains.</p>
			<div
				class="mt-3 flex min-h-10 flex-wrap items-center gap-1.5 rounded-xl border border-input bg-background p-1.5 focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/20"
			>
				{#each saveButtonSettings.domains as domain (domain)}
					<span
						class="inline-flex max-w-full items-center gap-1 rounded-full bg-muted py-1 pr-1 pl-2.5 text-xs"
					>
						<span class="truncate">{domain}</span>
						<button
							type="button"
							class="rounded-full p-0.5 text-muted-foreground hover:bg-background hover:text-foreground"
							aria-label={`Remove ${domain}`}
							onclick={() => removeDomain(domain)}><X class="size-3" /></button
						>
					</span>
				{/each}
				<input
					id="save-button-domain"
					class="h-7 min-w-28 flex-1 bg-transparent px-1 text-sm outline-none placeholder:text-muted-foreground"
					placeholder="example.com"
					bind:value={domainInput}
					oninput={() => (error = '')}
					onkeydown={handleDomainKeydown}
					aria-invalid={!!error}
					aria-describedby="save-button-domain-help"
				/>
			</div>
			<p
				id="save-button-domain-help"
				class="mt-1.5 text-xs {error ? 'text-destructive' : 'text-muted-foreground'}"
			>
				{error || 'Type a domain or paste a URL, then press Enter.'}
			</p>
			{#if currentDomain && !isCurrentsHost(currentDomain)}
				<div class="mt-3 flex items-center justify-between gap-2">
					<Button
						variant="outline"
						size="sm"
						disabled={currentAdded}
						onclick={() => addDomain(currentDomain)}
						><Plus class="size-4" /> Add this website</Button
					>
					<span class="truncate text-xs text-muted-foreground" title={currentDomain}>
						{currentDomain}
					</span>
				</div>
			{/if}
		</section>
	{/if}
</div>
