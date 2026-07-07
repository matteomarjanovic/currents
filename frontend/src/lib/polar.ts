import { PUBLIC_POLAR_PRODUCT_MONTHLY, PUBLIC_POLAR_PRODUCT_YEARLY } from '$env/static/public';
import { PolarEmbedCheckout } from '@polar-sh/checkout/embed';
import { mode } from 'mode-watcher';
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
	const theme = mode.current === 'dark' ? 'dark' : 'light';
	const checkoutPromise = PolarEmbedCheckout.create(url, { theme });
	// The embed stamps `color-scheme: auto` (an invalid value) on its iframe, and
	// the browser only composites a cross-origin iframe transparently when the
	// element's color scheme matches the embedding document's — so in dark mode
	// the checkout gets an opaque background. Set the matching scheme before the
	// checkout document loads (create() appends the iframe synchronously; after
	// load it's too late). `theme` tracks the same mode that drives the app's
	// own color-scheme, so they always agree.
	const iframe = document.querySelector<HTMLIFrameElement>(`iframe[src^="${url}"]`);
	if (iframe) iframe.style.colorScheme = theme;
	const checkout = await checkoutPromise;
	checkout.addEventListener('success', () => void celebrate());
}
