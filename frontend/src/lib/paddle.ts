import {
	PUBLIC_PADDLE_CLIENT_TOKEN,
	PUBLIC_PADDLE_ENVIRONMENT,
	PUBLIC_PADDLE_PRICE_MONTHLY,
	PUBLIC_PADDLE_PRICE_YEARLY
} from '$env/static/public';
import { CheckoutEventNames, initializePaddle, type Paddle } from '@paddle/paddle-js';
import { mode } from 'mode-watcher';
import { loadSupporterStatus, supporter } from '$lib/stores/supporter.svelte';

export const PADDLE_PRICE_MONTHLY = PUBLIC_PADDLE_PRICE_MONTHLY;
export const PADDLE_PRICE_YEARLY = PUBLIC_PADDLE_PRICE_YEARLY;
export const paddleConfigured = !!(
	PUBLIC_PADDLE_CLIENT_TOKEN &&
	PUBLIC_PADDLE_PRICE_MONTHLY &&
	PUBLIC_PADDLE_PRICE_YEARLY
);

// initializePaddle refuses a second call, so the instance is a module singleton.
let paddlePromise: Promise<Paddle | undefined> | null = null;
function getPaddle() {
	paddlePromise ??= initializePaddle({
		token: PUBLIC_PADDLE_CLIENT_TOKEN,
		environment: PUBLIC_PADDLE_ENVIRONMENT === 'production' ? 'production' : 'sandbox',
		eventCallback: (event) => {
			// Provisioning happens via the appview's Paddle webhook; the completed
			// event just tells us to poll until the mirror catches up (usually the
			// first attempt) so the UI unlocks while the overlay is still open.
			if (event.name === CheckoutEventNames.CHECKOUT_COMPLETED) void refreshUntilActive();
		}
	});
	return paddlePromise;
}

async function refreshUntilActive() {
	for (const delay of [1000, 2000, 4000, 8000]) {
		await new Promise((r) => setTimeout(r, delay));
		await loadSupporterStatus();
		if (supporter.active) return;
	}
}

// Opens the overlay checkout for a supporter subscription. custom_data.did is
// how the webhook maps the Paddle subscription back to the Currents user.
export async function openSupporterCheckout(priceId: string, did: string) {
	const paddle = await getPaddle();
	paddle?.Checkout.open({
		items: [{ priceId, quantity: 1 }],
		customData: { did },
		settings: {
			variant: 'one-page',
			theme: mode.current === 'dark' ? 'dark' : 'light'
		}
	});
}
