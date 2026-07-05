import { apiFetch } from '$lib/api';

// Supporter-tier entitlement (semantic library search + find-similar in
// library), mirrored from the server. `active` is what the gate enforces —
// when the backend has no Polar configured it reports everyone active, so
// the client gate stays dormant. `subscribed` is the real Polar subscription
// state, driving the settings dialog's subscription section. The server
// enforces the gate regardless (403 SupporterRequired).
export const supporter = $state({
	active: false,
	subscribed: false,
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

export async function loadSupporterStatus() {
	try {
		const res = await apiFetch('/api/supporter/status');
		if (!res.ok) return;
		const data = (await res.json()) as { active?: boolean; subscribed?: boolean };
		supporter.active = data.active === true;
		supporter.subscribed = data.subscribed === true;
		supporter.loaded = true;
	} catch {
		// best-effort; unloaded state just re-checks on the next gate hit
	}
}
