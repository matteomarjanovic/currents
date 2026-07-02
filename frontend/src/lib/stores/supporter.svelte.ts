import { apiFetch } from '$lib/api';

// Supporter-tier entitlement (semantic library search + find-similar in
// library), mirrored from the server. `active` is what the gate enforces —
// when the backend has no Paddle configured it reports everyone active, so
// the client gate stays dormant. `subscribed` is the real Paddle subscription
// state, driving the settings dialog's subscription section. The server
// enforces the gate regardless (403 SupporterRequired).
export const supporter = $state({
	active: false,
	subscribed: false,
	loaded: false
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
