<script lang="ts">
	import { untrack } from 'svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { Button } from '$lib/components/ui/button';
	import { toast } from 'svelte-sonner';
	import { apiFetch } from '$lib/api';
	import { settingsDialog, type SettingsSection } from '$lib/stores/settings.svelte';
	import { supporter, loadSupporterStatus } from '$lib/stores/supporter.svelte';
	import { role, loadRole, previewGated, canSeePreviewFeatures } from '$lib/stores/role.svelte';
	import {
		modPrefs,
		modPrefsLoaded,
		loadModerationPrefs,
		setAdult,
		setAi,
		type AdultKey,
		type AdultVisibility,
		type AiVisibility
	} from '$lib/stores/moderation-prefs.svelte';
	import SupporterPlans from '$lib/components/supporter-plans.svelte';
	import SupporterBadge from '$lib/components/supporter-badge.svelte';
	import ShieldIcon from '@lucide/svelte/icons/shield';
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import ExternalLink from '@lucide/svelte/icons/external-link';

	// Settings live in a dialog (not a page) so they open from every mode —
	// explore's top bar, organize's sidebar, and the blurred-media overlays.
	// Open state is the global settingsDialog store; this is mounted once in
	// the root layout.

	const NAV: { key: SettingsSection; name: string; icon: typeof ShieldIcon }[] = [
		{ key: 'moderation', name: 'Moderation', icon: ShieldIcon },
		{ key: 'subscription', name: 'Subscription', icon: SparklesIcon }
	];
	// While the preview gate is on, the subscription section is moderator-only.
	let nav = $derived(canSeePreviewFeatures() ? NAV : NAV.filter((n) => n.key !== 'subscription'));
	let section = $derived(
		nav.some((n) => n.key === settingsDialog.section) ? settingsDialog.section : 'moderation'
	);
	let activeName = $derived(NAV.find((n) => n.key === section)?.name ?? '');

	// Refresh server-backed state each time the dialog opens (prefs only once —
	// they're synced optimistically on change).
	$effect(() => {
		const isOpen = settingsDialog.open;
		untrack(() => {
			if (!isOpen) return;
			if (!modPrefsLoaded.value) void loadModerationPrefs();
			if (previewGated && !role.loaded) void loadRole();
			void loadSupporterStatus();
		});
	});

	const ADULT_CATEGORIES: { key: AdultKey; label: string; description: string }[] = [
		{
			key: 'porn',
			label: 'Porn',
			description: 'Sexually explicit imagery.'
		},
		{
			key: 'sexual',
			label: 'Sexual',
			description: 'Sexually suggestive content that stops short of explicit.'
		},
		{
			key: 'nudity',
			label: 'Nudity',
			description: 'Non-sexual nudity (artistic, documentary, fashion).'
		},
		{
			key: 'graphicMedia',
			label: 'Graphic violence',
			description: 'Gore, injury, or other distressing imagery.'
		}
	];

	const ADULT_OPTIONS: { val: AdultVisibility; label: string }[] = [
		{ val: 'show', label: 'Show' },
		{ val: 'blur', label: 'Blur' },
		{ val: 'hide', label: 'Hide' }
	];

	const AI_OPTIONS: { val: AiVisibility; label: string }[] = [
		{ val: 'show', label: 'Show' },
		{ val: 'hide', label: 'Hide' }
	];

	let portalLoading = $state(false);
	async function openPortal() {
		if (portalLoading) return;
		portalLoading = true;
		// Open the tab synchronously with the click so popup blockers allow it,
		// then point it at the freshly minted portal session.
		const tab = window.open('about:blank', '_blank');
		try {
			const res = await apiFetch('/api/supporter/portal', { method: 'POST' });
			if (!res.ok) throw new Error(`${res.status}`);
			const { url } = (await res.json()) as { url: string };
			if (tab) tab.location.href = url;
			else window.open(url, '_blank', 'noopener');
		} catch {
			tab?.close();
			toast.error("Couldn't open the billing portal");
		} finally {
			portalLoading = false;
		}
	}
</script>

<Dialog.Root bind:open={settingsDialog.open}>
	<Dialog.Content
		class="overflow-hidden p-0 md:max-h-[500px] md:max-w-[700px] lg:max-w-[800px]"
		trapFocus={false}
	>
		<Dialog.Title class="sr-only">Settings</Dialog.Title>
		<Dialog.Description class="sr-only">Manage your Currents settings.</Dialog.Description>
		<Sidebar.Provider class="items-start">
			<Sidebar.Root collapsible="none" class="hidden md:flex">
				<Sidebar.Content>
					<Sidebar.Group>
						<Sidebar.GroupLabel>Settings</Sidebar.GroupLabel>
						<Sidebar.GroupContent>
							<Sidebar.Menu>
								{#each nav as item (item.key)}
									<Sidebar.MenuItem>
										<Sidebar.MenuButton
											isActive={section === item.key}
											onclick={() => (settingsDialog.section = item.key)}
										>
											<item.icon />
											<span>{item.name}</span>
										</Sidebar.MenuButton>
									</Sidebar.MenuItem>
								{/each}
							</Sidebar.Menu>
						</Sidebar.GroupContent>
					</Sidebar.Group>
				</Sidebar.Content>
			</Sidebar.Root>
			<main class="flex h-[480px] flex-1 flex-col overflow-hidden">
				<header class="flex h-14 shrink-0 items-center px-4">
					<!-- The nav sidebar is hidden below md; switch sections here instead. -->
					<div class="inline-flex rounded-md border border-border p-0.5 md:hidden">
						{#each nav as item (item.key)}
							<button
								type="button"
								onclick={() => (settingsDialog.section = item.key)}
								class="rounded px-3 py-1.5 text-xs font-medium transition-colors {section ===
								item.key
									? 'bg-foreground text-background'
									: 'text-muted-foreground hover:bg-muted'}"
							>
								{item.name}
							</button>
						{/each}
					</div>
					<h2 class="hidden text-base font-semibold md:block">{activeName}</h2>
				</header>
				<div class="flex flex-1 flex-col gap-8 overflow-y-auto p-4 pt-1">
					{#if section === 'moderation'}
						<p class="text-sm text-muted-foreground">
							Synced to your account across devices. <b>Hide</b> filters matching saves out of every
							feed entirely. <b>Blur</b> shows a click-to-reveal warning. <b>Show</b> renders normally.
						</p>

						<section class="flex flex-col gap-4">
							<div>
								<h3 class="text-sm font-medium">Adult content</h3>
								<p class="text-sm text-muted-foreground">
									Set how each label is treated independently.
								</p>
							</div>
							<div class="flex flex-col gap-3">
								{#each ADULT_CATEGORIES as cat (cat.key)}
									<div
										class="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 sm:flex-row sm:items-center sm:justify-between"
									>
										<div class="flex flex-col gap-0.5">
											<span class="text-sm font-medium">{cat.label}</span>
											<span class="text-xs text-muted-foreground">{cat.description}</span>
										</div>
										<div
											class="inline-flex shrink-0 self-start rounded-md border border-border p-0.5 sm:self-auto"
										>
											{#each ADULT_OPTIONS as opt (opt.val)}
												{@const active = modPrefs[cat.key] === opt.val}
												<button
													type="button"
													disabled={!modPrefsLoaded.value}
													onclick={() => setAdult(cat.key, opt.val)}
													class="rounded px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50 {active
														? 'bg-foreground text-background'
														: 'text-muted-foreground hover:bg-muted'}"
												>
													{opt.label}
												</button>
											{/each}
										</div>
									</div>
								{/each}
							</div>
						</section>

						<section class="flex flex-col gap-4">
							<div>
								<h3 class="text-sm font-medium">AI-generated content</h3>
								<p class="text-sm text-muted-foreground">
									How should images detected as AI-generated appear?
								</p>
							</div>
							<div
								class="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 sm:flex-row sm:items-center sm:justify-between"
							>
								<div class="flex flex-col gap-0.5">
									<span class="text-sm font-medium">AI-generated</span>
									<span class="text-xs text-muted-foreground">
										When shown, a small "AI" badge appears in the corner.
									</span>
								</div>
								<div
									class="inline-flex shrink-0 self-start rounded-md border border-border p-0.5 sm:self-auto"
								>
									{#each AI_OPTIONS as opt (opt.val)}
										{@const active = modPrefs.aiGenerated === opt.val}
										<button
											type="button"
											disabled={!modPrefsLoaded.value}
											onclick={() => setAi(opt.val)}
											class="rounded px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50 {active
												? 'bg-foreground text-background'
												: 'text-muted-foreground hover:bg-muted'}"
										>
											{opt.label}
										</button>
									{/each}
								</div>
							</div>
						</section>
					{:else}
						<section class="flex flex-col gap-4">
							<p class="text-sm text-muted-foreground">
								Supporters unlock semantic search and visual similarity search in their library —
								and keep Currents running and independent.
							</p>
							{#if !supporter.loaded}
								<p class="text-sm text-muted-foreground">Loading…</p>
							{:else if supporter.subscribed}
								<div
									class="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4"
								>
									<div class="flex flex-col gap-0.5">
										<span class="flex items-center gap-2 text-sm font-medium">
											<SupporterBadge class="size-5" />
											You're a supporter
										</span>
										<span class="text-xs text-muted-foreground">
											Thank you for keeping Currents running.
										</span>
									</div>
									<Button variant="outline" size="sm" disabled={portalLoading} onclick={openPortal}>
										Manage
										<ExternalLink class="size-3.5" />
									</Button>
								</div>
								<p class="text-xs text-muted-foreground">
									Invoices, payment method, plan changes, and cancellation are handled in the Paddle
									billing portal.
								</p>
							{:else}
								<SupporterPlans onCheckoutOpen={() => (settingsDialog.open = false)} />
							{/if}
						</section>
					{/if}
				</div>
			</main>
		</Sidebar.Provider>
	</Dialog.Content>
</Dialog.Root>
