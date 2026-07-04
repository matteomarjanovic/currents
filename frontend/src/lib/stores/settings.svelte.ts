// Global open state for the settings dialog, so any surface (top bar, organize
// sidebar, blurred-media overlays) can open it. Mounted once in the root layout.
export type SettingsSection = 'moderation' | 'subscription' | 'account';

export const settingsDialog = $state({
	open: false,
	section: 'account' as SettingsSection
});

export function openSettings(section: SettingsSection = 'account') {
	settingsDialog.section = section;
	settingsDialog.open = true;
}
