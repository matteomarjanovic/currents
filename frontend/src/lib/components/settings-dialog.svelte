<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Button } from '$lib/components/ui/button';
	import { toast } from 'svelte-sonner';
	import { apiFetch } from '$lib/api';
	import { isNative } from '$lib/platform';
	import { clearAuthToken } from '$lib/auth-storage';
	import { auth } from '$lib/stores/auth.svelte';
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
	import { resolve } from '$app/paths';
	import ShieldIcon from '@lucide/svelte/icons/shield';
	import CreditCardIcon from '@lucide/svelte/icons/credit-card';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import UserIcon from '@lucide/svelte/icons/user';

	// Settings live in a dialog (not a page) so they open from every mode —
	// explore's top bar, organize's sidebar, and the blurred-media overlays.
	// Open state is the global settingsDialog store; this is mounted once in
	// the root layout.

	const NAV: { key: SettingsSection; name: string; icon: typeof ShieldIcon }[] = [
		{ key: 'account', name: 'Account', icon: UserIcon },
		{ key: 'subscription', name: 'Subscription', icon: CreditCardIcon },
		{ key: 'moderation', name: 'Moderation', icon: ShieldIcon }
	];
	// While the preview gate is on, the subscription section is moderator-only.
	let nav = $derived(canSeePreviewFeatures() ? NAV : NAV.filter((n) => n.key !== 'subscription'));
	let section = $derived(
		nav.some((n) => n.key === settingsDialog.section) ? settingsDialog.section : 'account'
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

	let deleteOpen = $state(false);
	let deletePds = $state(false);
	let deleting = $state(false);

	async function confirmDeleteAccount() {
		if (deleting) return;
		deleting = true;
		try {
			const res = await apiFetch('/api/account/delete', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ deletePdsData: deletePds })
			});
			if (res.status === 409) {
				toast.error('Cancel your supporter subscription first.');
				deleteOpen = false;
				void loadSupporterStatus();
				return;
			}
			if (!res.ok) throw new Error(`${res.status}`);
			// The server wiped the account and cleared the session cookie.
			auth.user = null;
			if (isNative()) {
				await clearAuthToken();
				auth.checked = true;
				deleteOpen = false;
				settingsDialog.open = false;
				await goto('/');
			} else {
				// Full reload resets every client store, mirroring the web logout flow.
				window.location.href = '/';
			}
		} catch {
			toast.error("Couldn't delete your account. Please try again.");
		} finally {
			deleting = false;
		}
	}
</script>

<Dialog.Root bind:open={settingsDialog.open}>
	<Dialog.Content
		class="h-[calc(100dvh-3rem)] overflow-hidden p-0 md:h-auto md:max-h-[500px] md:max-w-[700px] lg:max-w-[800px]"
		trapFocus={false}
	>
		<Dialog.Title class="sr-only">Settings</Dialog.Title>
		<Dialog.Description class="sr-only">Manage your Currents settings.</Dialog.Description>
		<Sidebar.Provider class="h-full min-h-0 items-start">
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
			<main class="flex h-full flex-1 flex-col overflow-hidden md:h-[480px]">
				<header
					class="flex shrink-0 flex-col gap-3 px-4 pt-4 pb-1 md:h-14 md:flex-row md:items-center md:py-0"
				>
					<h2 class="text-base font-semibold md:hidden">Settings</h2>
					<!-- The nav sidebar is hidden below md; switch sections here instead. -->
					<div class="inline-flex self-start rounded-md border border-border p-0.5 md:hidden">
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
					{:else if section === 'subscription'}
						<section class="flex flex-col gap-4">
							<p class="text-sm text-muted-foreground">
								Currents is an independent, ad-free project: it's funded by its supporters, not by
								your data. Supporting it also unlocks a set of extra perks, like semantic and visual
								search in your library.
								<a
									class="underline underline-offset-4 hover:text-foreground"
									href={resolve('/support-currents-project')}
									onclick={() => (settingsDialog.open = false)}
								>
									Learn more about supporting the project</a
								>.
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
									Invoices, payment method, plan changes, and cancellation are handled in the Polar
									billing portal.
								</p>
							{:else}
								<SupporterPlans onCheckoutOpen={() => (settingsDialog.open = false)} />
							{/if}
						</section>
					{:else}
						<section class="flex flex-col gap-4">
							<div class="flex items-center gap-3 rounded-lg border border-border bg-card p-4">
								<Avatar.Root size="default">
									{#if auth.user?.avatar}
										<Avatar.Image
											src={auth.user.avatar}
											alt={auth.user.displayName ?? auth.user.handle}
										/>
									{/if}
									<Avatar.Fallback>
										<UserIcon class="size-4" />
									</Avatar.Fallback>
								</Avatar.Root>
								<div class="flex flex-col gap-0.5">
									<span class="text-sm font-medium">
										{auth.user?.displayName || auth.user?.handle}
									</span>
									<span class="text-xs text-muted-foreground">@{auth.user?.handle}</span>
								</div>
							</div>

							{#if supporter.subscribed}
								<div
									class="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-4"
								>
									<div class="flex flex-col gap-0.5">
										<span class="text-sm font-medium">Active subscription</span>
										<span class="text-xs text-muted-foreground">
											Cancel your supporter subscription in the billing portal before deleting your
											account.
										</span>
									</div>
									<Button variant="outline" size="sm" disabled={portalLoading} onclick={openPortal}>
										Manage
										<ExternalLink class="size-3.5" />
									</Button>
								</div>
							{/if}

							<div class="flex flex-col gap-3 rounded-lg border border-destructive/30 bg-card p-4">
								<div class="flex flex-col gap-0.5">
									<span class="text-sm font-medium">Delete account</span>
									<span class="text-xs text-muted-foreground">
										Removes your profile, collections, and saves from Currents and stops indexing
										your data. What happens to the records in your AT Protocol repo is up to you.
									</span>
								</div>
								<Button
									variant="destructive"
									size="sm"
									class="self-start"
									disabled={!supporter.loaded || supporter.subscribed}
									onclick={() => {
										deletePds = false;
										deleteOpen = true;
									}}
								>
									Delete account
								</Button>
							</div>
						</section>
					{/if}
				</div>
			</main>
		</Sidebar.Provider>
	</Dialog.Content>
</Dialog.Root>

<AlertDialog.Root bind:open={deleteOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete your account?</AlertDialog.Title>
			<AlertDialog.Description>
				This removes your profile, collections, and saves from Currents and stops indexing your
				data.
				{#if deletePds}
					Your AT Protocol repo will be scrubbed too — nothing will be left to restore.
				{:else}
					Your records stay in your AT Protocol repo — sign in again anytime to restore them.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<label class="flex items-start gap-2 text-sm">
			<input
				type="checkbox"
				bind:checked={deletePds}
				disabled={deleting}
				class="mt-0.5 accent-destructive"
			/>
			<span>
				Also permanently delete all Currents records from my AT Protocol repo. This cannot be
				undone.
				{#if deletePds}
					<span class="mt-1 block text-xs text-muted-foreground">
						Removal happens in the background and can take a while for large accounts (your data
						server limits deletions per hour).
					</span>
				{/if}
			</span>
		</label>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action
				onclick={confirmDeleteAccount}
				disabled={deleting}
				class="text-destructive-foreground bg-destructive hover:bg-destructive/90"
			>
				{deleting ? 'Deleting…' : 'Delete account'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
