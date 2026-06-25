<script lang="ts">
	import { onDestroy, untrack } from 'svelte';
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
	import Images from '@lucide/svelte/icons/images';
	import { isNative } from '$lib/platform';
	import type { ActorProfileView } from '$lib/types';

	const native = isNative();

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
	// The chosen image as a single File (from the web file input or the native gallery picker).
	let avatarFile = $state<File | undefined>(undefined);
	let bannerFile = $state<File | undefined>(undefined);

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
		avatarFile = undefined;
		bannerFile = undefined;
		if (avatarInput) avatarInput.value = '';
		if (bannerInput) bannerInput.value = '';
		error = null;
	}

	// Only re-populate the form when the dialog opens — untrack so the reads inside resetForm
	// (e.g. avatarObjectUrl via the revoke helpers) don't make this effect re-run and wipe a
	// freshly-picked image the moment its object URL is created.
	// Only re-populate the form when the dialog opens — untrack so the reads inside resetForm
	// (e.g. avatarObjectUrl via the revoke helpers) don't make this effect re-run and wipe a
	// freshly-picked image the moment its object URL is created.
	$effect(() => {
		if (open) untrack(() => resetForm());
	});

	function applyImportedImage(
		kind: 'avatar' | 'banner',
		preview: string | undefined,
		blob: RepoBlobRef | undefined
	) {
		if (kind === 'avatar') {
			revokeAvatarObjectUrl();
			avatarFile = undefined;
			if (avatarInput) avatarInput.value = '';
			avatarPreview = preview ?? '';
			avatarBlob = blob ? JSON.stringify(blob) : '';
			removeAvatar = !blob;
			return;
		}
		revokeBannerObjectUrl();
		bannerFile = undefined;
		if (bannerInput) bannerInput.value = '';
		bannerPreview = preview ?? '';
		bannerBlob = blob ? JSON.stringify(blob) : '';
		removeBanner = !blob;
	}

	function setAvatarFile(file: File) {
		revokeAvatarObjectUrl();
		avatarFile = file;
		avatarObjectUrl = URL.createObjectURL(file);
		avatarPreview = avatarObjectUrl;
		avatarBlob = '';
		removeAvatar = false;
	}

	function setBannerFile(file: File) {
		revokeBannerObjectUrl();
		bannerFile = file;
		bannerObjectUrl = URL.createObjectURL(file);
		bannerPreview = bannerObjectUrl;
		bannerBlob = '';
		removeBanner = false;
	}

	function onAvatarInput(e: Event) {
		const file = (e.currentTarget as HTMLInputElement).files?.[0];
		if (file) setAvatarFile(file);
	}

	function onBannerInput(e: Event) {
		const file = (e.currentTarget as HTMLInputElement).files?.[0];
		if (file) setBannerFile(file);
	}

	// Native uses the gallery picker (the web file input returns a content:// File the
	// Android WebView can't read), mirroring the image-upload page.
	async function pickProfileImage(kind: 'avatar' | 'banner') {
		try {
			const { Camera, CameraResultType, CameraSource } = await import('@capacitor/camera');
			const photo = await Camera.getPhoto({
				resultType: CameraResultType.Uri,
				source: CameraSource.Photos,
				quality: 90,
				correctOrientation: true,
				allowEditing: false
			});
			if (!photo.webPath) return;
			const blob = await (await fetch(photo.webPath)).blob();
			const ext = (photo.format ?? 'jpeg').toLowerCase();
			const file = new File([blob], `${kind}-${Date.now()}.${ext}`, {
				type: blob.type || `image/${ext}`,
				lastModified: Date.now()
			});
			if (kind === 'avatar') setAvatarFile(file);
			else setBannerFile(file);
		} catch (err) {
			// `cancelled` is the user dismissing the picker — not an error worth surfacing.
			const msg = err instanceof Error ? err.message : String(err);
			console.warn('[profile-pick] error', msg);
			if (!/cancel/i.test(msg)) toast.error('Could not load the selected image.');
		}
	}

	function clearAvatar() {
		revokeAvatarObjectUrl();
		avatarFile = undefined;
		if (avatarInput) avatarInput.value = '';
		avatarPreview = '';
		avatarBlob = '';
		removeAvatar = true;
	}

	function clearBanner() {
		revokeBannerObjectUrl();
		bannerFile = undefined;
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
			if (avatarFile) form.set('avatar', avatarFile, avatarFile.name);
			if (bannerFile) form.set('banner', bannerFile, bannerFile.name);

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
					<Label>Avatar</Label>
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
					{#if native}
						<Button
							type="button"
							variant="secondary"
							size="sm"
							class="w-full"
							onclick={() => pickProfileImage('avatar')}
							disabled={submitting || importing}
						>
							<Images class="size-4" />
							Choose from gallery
						</Button>
					{:else}
						<Input
							id="profile-avatar"
							type="file"
							accept="image/png,image/jpeg"
							bind:ref={avatarInput}
							onchange={onAvatarInput}
							disabled={submitting || importing}
						/>
					{/if}
					<Button
						type="button"
						variant="ghost"
						size="sm"
						class="w-full"
						onclick={clearAvatar}
						disabled={submitting || importing || (!avatarPreview && !avatarFile && !avatarBlob)}
					>
						Remove avatar
					</Button>
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
						{#if native}
							<Button
								type="button"
								variant="secondary"
								size="sm"
								class="w-full"
								onclick={() => pickProfileImage('banner')}
								disabled={submitting || importing}
							>
								<Images class="size-4" />
								Choose from gallery
							</Button>
						{:else}
							<Input
								id="profile-banner"
								type="file"
								accept="image/png,image/jpeg"
								bind:ref={bannerInput}
								onchange={onBannerInput}
								disabled={submitting || importing}
							/>
						{/if}
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onclick={clearBanner}
							disabled={submitting || importing || (!bannerPreview && !bannerFile && !bannerBlob)}
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
