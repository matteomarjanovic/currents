import { toast } from 'svelte-sonner';
import { appviewUrl } from '$lib/api';
import { auth } from '$lib/stores/auth.svelte';
import { isNative } from '$lib/platform';

// Re-run the OAuth flow for the currently logged-in user to pick up newly
// requested scopes (here: the uploadBlob rpc: scope that enables direct-to-PDS
// uploads). A full-page POST to /oauth/login → PDS authorization → back here.
export function reauthorize(returnTo: string = location.pathname + location.search) {
	const did = auth.user?.did;
	if (!did) return;

	// Native OAuth runs in Capacitor's browser and must return through the custom
	// URL scheme so the app can store the refreshed session token. Use the DID as
	// the login hint: it is stable and is the identifier accepted by OAuth even
	// when the cached profile handle is unavailable.
	if (isNative()) {
		const url = new URL(`${PUBLIC_APPVIEW_URL}/oauth/login`);
		url.searchParams.set('username', did);
		url.searchParams.set('return_to', 'currents://oauth-callback');
		void import('@capacitor/browser').then(({ Browser }) =>
			Browser.open({ url: url.toString(), presentationStyle: 'popover' })
		);
		return;
	}

	const form = document.createElement('form');
	form.method = 'POST';
	form.action = appviewUrl('/oauth/login');
	form.style.display = 'none';
	for (const [name, value] of [
		['username', did],
		['return_to', returnTo]
	]) {
		const input = document.createElement('input');
		input.type = 'hidden';
		input.name = name;
		input.value = value;
		form.appendChild(input);
	}
	document.body.appendChild(form);
	form.submit();
}

let prompted = false;

// Nudge the user to reconnect after a direct upload failed because their session
// predates the uploadBlob rpc: scope (there's no server-side fallback, so the
// upload didn't go through). Shown once per page session so a multi-file upload
// doesn't stack toasts; re-authorizing grants the scope and stops it firing.
export function promptUploadReauth() {
	if (prompted) return;
	prompted = true;
	toast('Reconnect to save', {
		description: 'Saving needs a quick reconnect to your data server. Reconnect and try again.',
		duration: 10000,
		action: { label: 'Reconnect', onClick: () => reauthorize() }
	});
}
