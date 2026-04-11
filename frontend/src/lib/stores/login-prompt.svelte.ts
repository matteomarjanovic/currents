export const loginPrompt = $state({ open: false });

export function promptLogin() {
	loginPrompt.open = true;
}
