// The paid native plugin is intentionally absent from web-only installs.
declare module '@capawesome-team/capacitor-secure-preferences' {
	export const SecurePreferences: {
		get(options: { key: string }): Promise<{ value?: string }>;
		set(options: { key: string; value: string }): Promise<void>;
		remove(options: { key: string }): Promise<void>;
	};
}
