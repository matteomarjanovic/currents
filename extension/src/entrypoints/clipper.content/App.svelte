<script lang="ts">
	import { clipper, hideClipper } from '../../lib/clipper-store.svelte';
	import LoginGate from './LoginGate.svelte';
	import SinglePicker from './SinglePicker.svelte';
	import MultiPicker from './MultiPicker.svelte';
	import SettingsPanel from './SettingsPanel.svelte';
	import UiImage from '../../lib/UiImage.svelte';
	import { provideViewNavigation } from '../../lib/view-navigation';
	import logo from '../../../../frontend/src/lib/assets/logo.svelte?raw';
	import { Button } from '$lib/components/ui/button';
	import * as Popover from '$lib/components/ui/popover';
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import LogOut from '@lucide/svelte/icons/log-out';
	import Settings from '@lucide/svelte/icons/settings';
	import User from '@lucide/svelte/icons/user';
	import X from '@lucide/svelte/icons/x';

	let profileOpen = $state(false);
	let settingsOpen = $state(false);
	let loggingOut = $state(false);
	let logoutError = $state('');
	let revealing = $state(true);
	let entering = $state(true);
	let panel = $state<HTMLDivElement>();
	let view = $state<HTMLDivElement>();
	const { navigate, reset: resetNavigation } = provideViewNavigation(() =>
		panel && view ? { panel, view } : undefined
	);
	const frontendUrl = import.meta.env.VITE_CURRENTS_FRONTEND_URL ?? 'https://currents.is';

	$effect(() => {
		clipper.session;
		resetNavigation();
		settingsOpen = false;
	});

	$effect(() => {
		if (!clipper.visible) {
			resetNavigation();
			revealing = true;
			entering = true;
			return;
		}
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
			revealing = false;
			entering = false;
			return;
		}
		const revealTimer = setTimeout(() => (revealing = false), 440);
		const enterTimer = setTimeout(() => (entering = false), 840);
		return () => {
			clearTimeout(revealTimer);
			clearTimeout(enterTimer);
		};
	});

	$effect(() => {
		if (clipper.closing) {
			resetNavigation();
			profileOpen = false;
		}
	});

	function close() {
		// A batch save runs inside the dialog; dismissing it would abandon the run.
		if (clipper.locked) return;
		profileOpen = false;
		hideClipper();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape' || e.defaultPrevented) return;
		if (profileOpen) profileOpen = false;
		else if (settingsOpen) void navigate(() => (settingsOpen = false), -1);
		else close();
	}

	async function logout() {
		loggingOut = true;
		logoutError = '';
		try {
			const result = await browser.runtime.sendMessage({ type: 'LOG_OUT' });
			if (!result.ok) throw new Error(result.error ?? 'Could not log out');
			profileOpen = false;
			clipper.authState = 'unauthenticated';
			clipper.userHandle = '';
			clipper.userDisplayName = '';
			clipper.userAvatar = '';
			clipper.collections = [];
			clipper.reauthNeeded = false;
		} catch (e) {
			logoutError = String(e);
		} finally {
			loggingOut = false;
		}
	}

	// Promote the panel into the browser top layer so page UI can't paint
	// over it, whatever z-index (or top layer) the page uses. Falls back to the
	// shadow host's max z-index where the Popover API is unavailable.
	function topLayer(node: HTMLElement) {
		try {
			node.showPopover?.();
		} catch {
			// already shown
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if clipper.visible}
	<div
		{@attach topLayer}
		popover="manual"
		class="fixed top-4 right-4 bottom-auto left-auto isolate z-50 m-0 w-[380px] max-w-[calc(100vw-2rem)] overflow-visible border-0 bg-transparent p-0 font-sans"
	>
		<div
			bind:this={panel}
			class="clipper-panel relative flex max-h-[calc(100vh-2rem)] flex-col gap-3 rounded-3xl bg-popover p-4 pt-3 text-sm text-popover-foreground shadow-lg ring-1 ring-foreground/10 {revealing
				? 'clipper-revealing'
				: ''} {entering ? 'clipper-entering' : ''} {clipper.closing ? 'clipper-closing' : ''}"
			role="dialog"
			aria-modal="false"
			aria-label={settingsOpen ? 'Currents settings' : 'Save to Currents'}
			inert={entering || clipper.closing}
			tabindex="-1"
		>
			<header class="relative flex min-h-8 shrink-0 items-center">
				<button
					type="button"
					aria-label="Currents"
					class="clipper-logo absolute top-1/2 left-0 h-4 w-24 -translate-y-1/2 cursor-default text-foreground"
				>
					{@html logo}
				</button>
				<div class="clipper-actions ml-auto flex items-center gap-1">
					{#if clipper.authState === 'authenticated'}
						<Popover.Root bind:open={profileOpen}>
							<Popover.Trigger disabled={clipper.locked || !clipper.userHandle}>
								{#snippet child({ props })}
									<button
										{...props}
										type="button"
										aria-label="Profile menu"
										class="grid size-6 place-items-center overflow-hidden rounded-full bg-muted"
									>
										{#if clipper.userAvatar}
											<UiImage
												src={clipper.userAvatar}
												class="size-full object-cover"
												loading="eager"
												onFail={() => (clipper.userAvatar = '')}
											/>
										{:else}
											<User class="size-4 text-muted-foreground" />
										{/if}
									</button>
								{/snippet}
							</Popover.Trigger>
							<Popover.Content
								side="bottom"
								align="end"
								sideOffset={8}
								portalProps={{ disabled: true }}
								class="w-64 gap-4 p-3"
							>
								<div class="flex min-w-0 items-center gap-3">
									<div
										class="grid size-12 shrink-0 place-items-center overflow-hidden rounded-full bg-muted"
									>
										{#if clipper.userAvatar}
											<UiImage
												src={clipper.userAvatar}
												class="size-full object-cover"
												loading="eager"
												onFail={() => (clipper.userAvatar = '')}
											/>
										{:else}
											<User class="size-6 text-muted-foreground" />
										{/if}
									</div>
									<div class="min-w-0">
										<p class="truncate text-base font-semibold">
											{clipper.userDisplayName || clipper.userHandle}
										</p>
										<p class="truncate text-sm text-muted-foreground">@{clipper.userHandle}</p>
									</div>
								</div>
								<div class="flex items-center gap-2">
									<Button
										href={`${frontendUrl}/profile/${encodeURIComponent(clipper.userHandle)}`}
										target="_blank"
										rel="noopener noreferrer"
										variant="outline"
										size="sm"
										class="min-w-0 flex-1"
										onclick={() => (profileOpen = false)}
										><ExternalLink class="size-4" /> Go to profile</Button
									>
									<Button
										variant="destructive"
										size="icon-sm"
										class="rounded-full"
										aria-label="Log out"
										title="Log out"
										disabled={loggingOut}
										onclick={logout}><LogOut class="size-4" /></Button
									>
								</div>
								{#if logoutError}<p class="text-xs text-destructive">{logoutError}</p>{/if}
							</Popover.Content>
						</Popover.Root>
					{/if}
					<div class="flex items-center -space-x-1">
						{#if !settingsOpen}
							<Button
								variant="ghost"
								size="icon-sm"
								aria-label="Settings"
								disabled={clipper.locked}
								onclick={() => {
									profileOpen = false;
									void navigate(() => (settingsOpen = true));
								}}
								class="text-muted-foreground"
							>
								<Settings />
							</Button>
						{/if}
						<Button
							variant="ghost"
							size="icon-sm"
							onclick={close}
							disabled={clipper.locked}
							aria-label="Close"
							class="text-muted-foreground"
						>
							<X />
						</Button>
					</div>
				</div>
			</header>

			<div class="relative flex min-h-0 flex-1 flex-col overflow-hidden">
				<div
					bind:this={view}
					class="clipper-view flex min-h-0 flex-1 flex-col gap-3 outline-none"
					tabindex="-1"
				>
					<!-- Keep the save form mounted so Back returns to the same selection. -->
					<div
						class="clipper-save-view {settingsOpen ? 'hidden' : 'contents'}"
						inert={settingsOpen}
					>
						{#key clipper.session}
							{#if clipper.authState === 'unauthenticated'}
								<LoginGate />
							{:else if clipper.mode === 'multi'}
								<MultiPicker />
							{:else}
								<SinglePicker />
							{/if}
						{/key}
					</div>
					{#if settingsOpen}
						<SettingsPanel onBack={() => void navigate(() => (settingsOpen = false), -1)} />
					{/if}
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	.clipper-revealing {
		animation: panel-reveal 440ms both;
	}

	.clipper-entering .clipper-logo {
		animation: logo-travel 800ms linear both;
	}

	.clipper-entering .clipper-actions {
		animation: actions-enter 370ms 430ms cubic-bezier(0.16, 1, 0.3, 1) both;
	}

	.clipper-entering .clipper-view {
		animation: content-enter 360ms 450ms cubic-bezier(0.22, 1, 0.36, 1) both;
	}

	.clipper-closing {
		animation: panel-fade-out 220ms ease-out both;
	}

	@keyframes panel-reveal {
		0% {
			clip-path: inset(0 0 calc(100% - 52px) calc(100% - 148px) round 24px);
			opacity: 0;
		}
		10% {
			opacity: 1;
		}
		18% {
			clip-path: inset(0 0 calc(100% - 52px) calc(100% - 148px) round 24px);
			animation-timing-function: cubic-bezier(0.16, 1, 0.3, 1);
		}
		100% {
			clip-path: inset(0 0 0 0 round 24px);
		}
	}

	@keyframes logo-travel {
		0% {
			left: calc(100% - 104px);
		}
		10% {
			left: calc(100% - 104px);
			animation-timing-function: cubic-bezier(0.16, 1, 0.3, 1);
		}
		52% {
			left: calc(50% - 48px);
			animation-timing-function: cubic-bezier(0.22, 1, 0.36, 1);
		}
		100% {
			left: 0;
		}
	}

	@keyframes actions-enter {
		from {
			opacity: 0;
			transform: translateX(-100px) scale(0.88);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}

	@keyframes content-enter {
		from {
			opacity: 0;
			transform: translateY(8px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}

	@keyframes panel-fade-out {
		to {
			opacity: 0;
			transform: translateY(-6px) scale(0.985);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.clipper-revealing,
		.clipper-entering .clipper-logo,
		.clipper-entering .clipper-actions,
		.clipper-entering .clipper-view,
		.clipper-closing {
			animation: none;
		}
	}
</style>
