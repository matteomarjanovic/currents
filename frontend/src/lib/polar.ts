import { PUBLIC_POLAR_PRODUCT_MONTHLY, PUBLIC_POLAR_PRODUCT_YEARLY } from '$env/static/public';
import { PolarEmbedCheckout } from '@polar-sh/checkout/embed';
import { mode } from 'mode-watcher';
import { toast } from 'svelte-sonner';
import { apiFetch } from '$lib/api';
import { loadSupporterStatus, supporter, supporterFlow } from '$lib/stores/supporter.svelte';

export const POLAR_PRODUCT_MONTHLY = PUBLIC_POLAR_PRODUCT_MONTHLY;
export const POLAR_PRODUCT_YEARLY = PUBLIC_POLAR_PRODUCT_YEARLY;
export const polarConfigured = !!(PUBLIC_POLAR_PRODUCT_MONTHLY && PUBLIC_POLAR_PRODUCT_YEARLY);

// After a completed checkout: wait for the webhook mirror to grant access,
// then open the thank-you dialog (whose close resumes any pending action).
async function celebrate() {
	await refreshUntilActive();
	supporterFlow.thanksOpen = true;
}

async function refreshUntilActive() {
	for (const delay of [1000, 2000, 4000, 8000]) {
		await new Promise((r) => setTimeout(r, delay));
		await loadSupporterStatus();
		if (supporter.active) return;
	}
}

// Opens the embedded overlay checkout for a supporter subscription. The
// appview creates the Polar checkout session, stamping the viewer's DID as
// the external customer id — that's how the webhook maps the subscription
// back to the Currents user. Provisioning happens via the appview's Polar
// webhook; the success event just tells us to poll until the mirror catches
// up (usually the first attempt) so the UI unlocks while the embed is open.
export async function openSupporterCheckout(productId: string) {
	const res = await apiFetch('/api/supporter/checkout', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ product: productId })
	});
	if (!res.ok) throw new Error(`checkout: ${res.status}`);
	const { url } = (await res.json()) as { url: string };
	// Render the embedded checkout in the app's current theme. Do NOT also stamp a
	// `color-scheme` on the checkout iframe: the embed leaves it at `auto`, which
	// lets the browser composite the cross-origin iframe transparently so its rgba
	// backdrop dims the page. Forcing `dark` mismatches Polar's checkout document
	// (which doesn't declare a dark scheme) and the browser paints an opaque white
	// backdrop over the whole overlay in dark mode.
	const checkout = await PolarEmbedCheckout.create(url, {
		theme: mode.current === 'dark' ? 'dark' : 'light'
	});
	checkout.addEventListener('success', () => void celebrate());
}

// Opens the Polar customer portal for an existing supporter to manage/cancel
// their subscription. Used by the settings dialog and the support page.
export async function openSupporterPortal() {
	// Open the tab synchronously with the click so popup blockers allow it,
	// then point it at the freshly minted portal session.
	const tab = window.open('about:blank', '_blank');
	try {
		const res = await apiFetch('/api/supporter/portal', { method: 'POST' });
		if (!res.ok) throw new Error(`${res.status}`);
		const { url } = (await res.json()) as { url: string };
		if (tab) tab.location.href = url;
		else window.open(url, '_blank', 'noopener');
	} catch {
		tab?.close();
		toast.error("Couldn't open the billing portal");
	}
}
