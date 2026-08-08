import { PUBLIC_APPVIEW_URL } from '$env/static/public';
import { toast } from 'svelte-sonner';
import { auth } from '$lib/stores/auth.svelte';

// Re-run the OAuth flow for the currently logged-in user to pick up newly
// requested scopes (here: the uploadBlob rpc: scope that enables direct-to-PDS
// uploads). A full-page POST to /oauth/login → PDS authorization → back here.
export function reauthorize(returnTo: string = location.pathname + location.search) {
	const handle = auth.user?.handle;
	if (!handle) return;
	const form = document.createElement('form');
	form.method = 'POST';
	form.action = `${PUBLIC_APPVIEW_URL}/oauth/login`;
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

// Nudge the user to reconnect after a direct upload fell back to a server-side
// upload because their session predates the uploadBlob rpc: scope. Shown once
// per page session so a multi-file upload doesn't stack toasts; re-authorizing
// grants the scope and stops it firing at all.
export function promptUploadReauth() {
	if (prompted) return;
	prompted = true;
	toast('Reconnect for faster uploads', {
		description:
			'Your uploads are going through our server. Reconnect your account to upload directly to your data server.',
		duration: 10000,
		action: { label: 'Reconnect', onClick: () => reauthorize() }
	});
}
