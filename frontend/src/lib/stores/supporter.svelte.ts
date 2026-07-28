import { apiFetch } from '$lib/api';
import { auth } from '$lib/stores/auth.svelte';
import { loginPrompt } from '$lib/stores/login-prompt.svelte';

// Supporter-tier entitlement (semantic library search + find-similar in
// library), mirrored from the server. `active` is what the gate enforces —
// when the backend has no Polar configured it reports everyone active, so
// the client gate stays dormant. `subscribed` is the real Polar subscription
// state, driving the settings dialog's subscription section. The server
// enforces the gate regardless (403 SupporterRequired).
export const supporter = $state({
	active: false,
	subscribed: false,
	// What's left of the color-search trial allowance (see requireColorSearch).
	// Only meaningful while `active` is false — the server reports 0 otherwise.
	colorTrialsLeft: 0,
	loaded: false
});

// Post-checkout flow: `pending` holds the supporter-tier action the paywall
// interrupted (set by the gate, cleared when it runs or the paywall is
// dismissed without buying); `thanksOpen` drives the thank-you dialog shown
// after a completed checkout, whose close resumes the pending action.
export const supporterFlow = $state({
	thanksOpen: false,
	pending: null as (() => void) | null
});

// Open state for the paywall dialog, mounted once in the root layout so any
// surface (explore color search, organize library search/find-similar) can
// raise it via requireSupporter.
export const supporterGate = $state({ open: false });

// A logged-out viewer has no account to attach a subscription to, so both gates
// ask them to sign in first — the paywall is only ever the second step.
function loggedOut(): boolean {
	if (auth.user) return false;
	loginPrompt.open = true;
	return true;
}

// Gate a supporter-tier action. Resolves true when the viewer is entitled;
// otherwise stashes `pending` on the supporter flow (resumed by the
// post-checkout thank-you dialog), opens the paywall, and resolves false.
export async function requireSupporter(pending?: () => void): Promise<boolean> {
	if (loggedOut()) return false;
	if (!supporter.loaded) await loadSupporterStatus();
	if (supporter.active) return true;
	supporterFlow.pending = pending ?? null;
	supporterGate.open = true;
	return false;
}

// Gate a color search. Unlike the other supporter features this one is
// sampleable: non-supporters spend a lifetime allowance of distinct query
// colors before the paywall. The server owns and enforces the count, so
// refresh it at the moment of the decision — a search made in another tab or
// on the phone has to be reflected here.
export async function requireColorSearch(pending?: () => void): Promise<boolean> {
	if (loggedOut()) return false;
	if (!supporter.loaded || !supporter.active) await loadSupporterStatus();
	if (supporter.active || supporter.colorTrialsLeft > 0) return true;
	supporterFlow.pending = pending ?? null;
	supporterGate.open = true;
	return false;
}

export async function loadSupporterStatus() {
	try {
		const res = await apiFetch('/api/supporter/status');
		if (!res.ok) return;
		const data = (await res.json()) as {
			active?: boolean;
			subscribed?: boolean;
			colorTrialsLeft?: number;
		};
		supporter.active = data.active === true;
		supporter.subscribed = data.subscribed === true;
		supporter.colorTrialsLeft = data.colorTrialsLeft ?? 0;
		supporter.loaded = true;
	} catch {
		// best-effort; unloaded state just re-checks on the next gate hit
	}
}
