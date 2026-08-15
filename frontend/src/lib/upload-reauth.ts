import { toast } from 'svelte-sonner';
import { appviewUrl } from '$lib/api';
import { auth } from '$lib/stores/auth.svelte';

// Re-run the OAuth flow for the currently logged-in user to pick up newly
// requested scopes (here: the uploadBlob rpc: scope that enables direct-to-PDS
// uploads). A full-page POST to /oauth/login → PDS authorization → back here.
export function reauthorize(returnTo: string = location.pathname + location.search) {
	const handle = auth.user?.handle;
	if (!handle) return;
	const form = document.createElement('form');
	form.method = 'POST';
	form.action = appviewUrl('/oauth/login');
	form.style.display = 'none';
	for (const [name, value] of [
		['username', handle],
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
