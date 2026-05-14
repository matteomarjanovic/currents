<script lang="ts">
	import { onDestroy } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Label } from '$lib/components/ui/label';
	import UserIcon from '@lucide/svelte/icons/user';
	import type { ActorProfileView } from '$lib/types';

	interface RepoBlobRef {
		$type?: string;
		ref: Record<string, string>;
		mimeType?: string;
		size?: number;
	}

	interface ProfileImportDraft {
		displayName?: string;
		description?: string;
		avatar?: string;
		banner?: string;
		avatarBlob?: RepoBlobRef;
		bannerBlob?: RepoBlobRef;
	}

	interface Props {
		open: boolean;
		profile: ActorProfileView;
		onSaved: (profile: ActorProfileView) => void;
	}

	let { open = $bindable(), profile, onSaved }: Props = $props();

	let displayName = $state('');
	let description = $state('');
	let pronouns = $state('');
	let website = $state('');
	let avatarPreview = $state('');
	let bannerPreview = $state('');
	let avatarBlob = $state('');
	let bannerBlob = $state('');
	let removeAvatar = $state(false);
	let removeBanner = $state(false);
	let submitting = $state(false);
	let importing = $state(false);
	let error = $state<string | null>(null);

	let avatarInput: HTMLInputElement | null = $state(null);
	let bannerInput: HTMLInputElement | null = $state(null);
	let avatarObjectUrl: string | null = $state(null);
	let bannerObjectUrl: string | null = $state(null);
	let avatarFiles = $state<FileList | undefined>(undefined);
	let bannerFiles = $state<FileList | undefined>(undefined);

	const initials = $derived(
		(displayName || profile.displayName || profile.handle || '?').trim().charAt(0).toUpperCase()
	);

	function revokeAvatarObjectUrl() {
		if (avatarObjectUrl) {
			URL.revokeObjectURL(avatarObjectUrl);
			avatarObjectUrl = null;
		}
	}

	function revokeBannerObjectUrl() {
		if (bannerObjectUrl) {
			URL.revokeObjectURL(bannerObjectUrl);
			bannerObjectUrl = null;
		}
	}

	function resetForm() {
		revokeAvatarObjectUrl();
		revokeBannerObjectUrl();
		displayName = profile.displayName ?? '';
		description = profile.description ?? '';
		pronouns = profile.pronouns ?? '';
		website = profile.website ?? '';
		avatarPreview = profile.avatar ?? '';
		bannerPreview = profile.banner ?? '';
		avatarBlob = '';
		bannerBlob = '';
		removeAvatar = false;
		removeBanner = false;
		avatarFiles = undefined;
		bannerFiles = undefined;
		if (avatarInput) avatarInput.value = '';
		if (bannerInput) bannerInput.value = '';
		error = null;
	}

	$effect(() => {
		if (open) resetForm();
	});

	function applyImportedImage(
		kind: 'avatar' | 'banner',
		preview: string | undefined,
		blob: RepoBlobRef | undefined
	) {
		if (kind === 'avatar') {
			revokeAvatarObjectUrl();
			avatarFiles = undefined;
			if (avatarInput) avatarInput.value = '';
			avatarPreview = preview ?? '';
			avatarBlob = blob ? JSON.stringify(blob) : '';
			removeAvatar = !blob;
			return;
		}
		revokeBannerObjectUrl();
		bannerFiles = undefined;
		if (bannerInput) bannerInput.value = '';
		bannerPreview = preview ?? '';
		bannerBlob = blob ? JSON.stringify(blob) : '';
		removeBanner = !blob;
	}

	function onAvatarChange() {
		const file = avatarFiles?.[0];
		if (!file) return;
		revokeAvatarObjectUrl();
		avatarObjectUrl = URL.createObjectURL(file);
		avatarPreview = avatarObjectUrl;
		avatarBlob = '';
		removeAvatar = false;
	}

	function onBannerChange() {
		const file = bannerFiles?.[0];
		if (!file) return;
		revokeBannerObjectUrl();
		bannerObjectUrl = URL.createObjectURL(file);
		bannerPreview = bannerObjectUrl;
		bannerBlob = '';
		removeBanner = false;
	}

	function clearAvatar() {
		revokeAvatarObjectUrl();
		avatarFiles = undefined;
		if (avatarInput) avatarInput.value = '';
		avatarPreview = '';
		avatarBlob = '';
		removeAvatar = true;
	}

	function clearBanner() {
		revokeBannerObjectUrl();
		bannerFiles = undefined;
		if (bannerInput) bannerInput.value = '';
		bannerPreview = '';
		bannerBlob = '';
		removeBanner = true;
	}

	async function importFromBluesky() {
		importing = true;
		error = null;
		try {
			const res = await apiFetch(`/api/profile/import-bluesky`);
			if (!res.ok) {
				error = (await res.text()).trim() || `Failed to import (${res.status}).`;
				return;
			}
			const draft: ProfileImportDraft = await res.json();
			displayName = draft.displayName ?? '';
			description = draft.description ?? '';
			applyImportedImage('avatar', draft.avatar, draft.avatarBlob);
			applyImportedImage('banner', draft.banner, draft.bannerBlob);
		} catch {
			error = 'Network error. Please try again.';
		} finally {
			importing = false;
		}
	}

	async function submit(e: Event) {
		e.preventDefault();
		submitting = true;
		error = null;
		try {
			const form = new FormData();
			form.set('displayName', displayName.trim());
			form.set('description', description.trim());
			form.set('pronouns', pronouns.trim());
			form.set('website', website.trim());
			if (removeAvatar) form.set('removeAvatar', 'true');
			if (removeBanner) form.set('removeBanner', 'true');
			if (avatarBlob) form.set('avatarBlob', avatarBlob);
			if (bannerBlob) form.set('bannerBlob', bannerBlob);
			if (avatarFiles?.[0]) form.set('avatar', avatarFiles[0], avatarFiles[0].name);
			if (bannerFiles?.[0]) form.set('banner', bannerFiles[0], bannerFiles[0].name);

			const res = await apiFetch(`/api/profile`, {
				method: 'PUT',
				body: form
			});
			if (!res.ok) {
				if (res.status === 401) auth.user = null;
				error = (await res.text()).trim() || `Failed to save (${res.status}).`;
				return;
			}
			onSaved(await res.json());
			toast.success('Profile updated');
			open = false;
		} catch {
			error = 'Network error. Please try again.';
		} finally {
			submitting = false;
		}
	}

	onDestroy(() => {
		revokeAvatarObjectUrl();
		revokeBannerObjectUrl();
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>Edit profile</Dialog.Title>
			<Dialog.Description>
				Update your Currents profile, or prefill the shared fields from Bluesky and review them
				before saving.
			</Dialog.Description>
		</Dialog.Header>

		<form onsubmit={submit} class="space-y-5">
			<div
				class="flex flex-col gap-3 rounded-3xl border border-border/70 bg-muted/30 p-3 sm:flex-row sm:items-center sm:justify-between"
			>
				<div>
					<p class="text-sm font-medium text-foreground">Import shared fields</p>
					<p class="text-sm text-muted-foreground">
						Copy display name, description, avatar, and banner from your Bluesky profile.
					</p>
				</div>
				<Button
					type="button"
					variant="outline"
					onclick={importFromBluesky}
					disabled={submitting || importing}
				>
					{importing ? 'Importing…' : 'Import from Bluesky'}
				</Button>
			</div>

			<div class="space-y-2">
				<Label for="profile-handle">Handle</Label>
				<Input id="profile-handle" value={`@${profile.handle}`} disabled />
			</div>

			<div class="grid gap-5 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
				<div class="space-y-3 rounded-3xl border border-border/70 p-4">
					<div class="space-y-2">
						<Label>Avatar</Label>
						<div class="flex items-center gap-4">
							<Avatar.Root class="size-20">
								{#if avatarPreview}
									<Avatar.Image src={avatarPreview} alt={displayName || profile.handle} />
								{/if}
								<Avatar.Fallback class="text-xl">
									{#if initials && initials !== '?'}
										{initials}
									{:else}
										<UserIcon class="size-6" />
									{/if}
								</Avatar.Fallback>
							</Avatar.Root>
							<div class="min-w-0 flex-1 space-y-2">
								<Input
									id="profile-avatar"
									type="file"
									accept="image/png,image/jpeg"
									bind:ref={avatarInput}
									bind:files={avatarFiles}
									onchange={onAvatarChange}
									disabled={submitting || importing}
								/>
								<Button
									type="button"
									variant="ghost"
									size="sm"
									onclick={clearAvatar}
									disabled={submitting ||
										importing ||
										(!avatarPreview && !avatarFiles?.[0] && !avatarBlob)}
								>
									Remove avatar
								</Button>
							</div>
						</div>
					</div>
				</div>

				<div class="space-y-3 rounded-3xl border border-border/70 p-4">
					<div class="space-y-2">
						<Label>Banner</Label>
						<div class="overflow-hidden rounded-3xl border border-border/70 bg-muted">
							{#if bannerPreview}
								<img
									src={bannerPreview}
									alt="Profile banner preview"
									class="h-28 w-full object-cover"
								/>
							{:else}
								<div class="flex h-28 items-center justify-center text-sm text-muted-foreground">
									No banner selected
								</div>
							{/if}
						</div>
						<Input
							id="profile-banner"
							type="file"
							accept="image/png,image/jpeg"
							bind:ref={bannerInput}
							bind:files={bannerFiles}
							onchange={onBannerChange}
							disabled={submitting || importing}
						/>
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onclick={clearBanner}
							disabled={submitting ||
								importing ||
								(!bannerPreview && !bannerFiles?.[0] && !bannerBlob)}
						>
							Remove banner
						</Button>
					</div>
				</div>
			</div>

			<div class="space-y-2">
				<Label for="profile-display-name">Display name</Label>
				<Input
					id="profile-display-name"
					bind:value={displayName}
					maxlength={64}
					disabled={submitting}
				/>
			</div>

			<div class="grid gap-4 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="profile-pronouns">Pronouns</Label>
					<Input id="profile-pronouns" bind:value={pronouns} maxlength={20} disabled={submitting} />
				</div>
				<div class="space-y-2">
					<Label for="profile-website">Website</Label>
					<Input
						id="profile-website"
						type="url"
						bind:value={website}
						placeholder="https://example.com"
						disabled={submitting}
					/>
				</div>
			</div>

			<div class="space-y-2">
				<Label for="profile-description">Description</Label>
				<Textarea
					id="profile-description"
					bind:value={description}
					maxlength={256}
					rows={5}
					disabled={submitting}
				/>
			</div>

			{#if error}
				<p class="text-sm text-destructive">{error}</p>
			{/if}

			<Dialog.Footer>
				<Button
					type="button"
					variant="outline"
					onclick={() => (open = false)}
					disabled={submitting || importing}
				>
					Cancel
				</Button>
				<Button type="submit" disabled={submitting || importing}>
					{submitting ? 'Saving…' : 'Save'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
