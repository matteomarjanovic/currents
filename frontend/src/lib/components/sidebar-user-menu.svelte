<script lang="ts">
	import { goto } from '$app/navigation';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { apiFetch } from '$lib/api';
	import { isNative } from '$lib/platform';
	import { clearAuthToken } from '$lib/auth-storage';
	import { auth } from '$lib/stores/auth.svelte';
	import { openSettings } from '$lib/stores/settings.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Avatar from '$lib/components/ui/avatar';
	import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down';
	import UserIcon from '@lucide/svelte/icons/user';
	import Settings from '@lucide/svelte/icons/settings';
	import LogOut from '@lucide/svelte/icons/log-out';

	// The avatar + account dropdown at the bottom of a mode sidebar (organize,
	// canvas). Must be rendered inside a Sidebar.Root.
	const sidebar = Sidebar.useSidebar();

	async function handleLogout() {
		if (isNative()) {
			try {
				await apiFetch('/oauth/logout');
			} catch {
				// best effort
			}
			await clearAuthToken();
			auth.user = null;
			auth.checked = true;
			goto('/');
		} else {
			window.location.href = `${PUBLIC_APPVIEW_URL}/oauth/logout`;
		}
	}

	let displayName = $derived(auth.user?.displayName || auth.user?.handle || '');
</script>

<Sidebar.Menu>
	<Sidebar.MenuItem>
		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Sidebar.MenuButton
						{...props}
						size="lg"
						class="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
					>
						<Avatar.Root class="size-8 rounded-lg">
							{#if auth.user?.avatar}
								<Avatar.Image src={auth.user.avatar} alt={displayName} />
							{/if}
							<Avatar.Fallback class="rounded-lg">
								<UserIcon class="size-4" />
							</Avatar.Fallback>
						</Avatar.Root>
						<div class="grid flex-1 text-left text-sm leading-tight">
							<span class="truncate font-medium">{displayName}</span>
							{#if auth.user}
								<span class="truncate text-xs text-muted-foreground">@{auth.user.handle}</span>
							{/if}
						</div>
						<ChevronsUpDown class="ml-auto size-4" />
					</Sidebar.MenuButton>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content side="top" align="start" class="w-56">
				{#if auth.user}
					<DropdownMenu.Item onclick={() => goto(`/profile/${auth.user!.handle}`)}>
						<UserIcon class="size-4" />
						Profile
					</DropdownMenu.Item>
				{/if}
				<DropdownMenu.Item
					onclick={() => {
						if (sidebar.isMobile) sidebar.setOpenMobile(false);
						openSettings();
					}}
				>
					<Settings class="size-4" />
					Settings
				</DropdownMenu.Item>
				<DropdownMenu.Separator />
				<DropdownMenu.Item onclick={handleLogout}>
					<LogOut class="size-4" />
					Log out
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</Sidebar.MenuItem>
</Sidebar.Menu>
