import { apiFetch } from '$lib/api';

// Shared follow/unfollow calls against the appview, used by the profile header
// and the Activity notifications tab. `scope-missing` means the user's OAuth
// session predates the follow scope and needs re-authorization.
export type FollowOutcome =
	| { status: 'ok'; uri: string }
	| { status: 'scope-missing' }
	| { status: 'error' };

export async function followUser(subject: string): Promise<FollowOutcome> {
	try {
		const res = await apiFetch(`/follow`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ subject })
		});
		if (res.ok) {
			const data = await res.json();
			return { status: 'ok', uri: data.uri };
		}
		if (res.status === 403) {
			const data = await res.json().catch(() => ({}));
			if (data.error === 'ScopeMissing') return { status: 'scope-missing' };
		}
		return { status: 'error' };
	} catch {
		return { status: 'error' };
	}
}

export async function unfollowUser(followUri: string): Promise<boolean> {
	const rkey = followUri.split('/').at(-1);
	try {
		const res = await apiFetch(`/follow/${rkey}`, {
			method: 'DELETE'
		});
		return res.ok;
	} catch {
		return false;
	}
}
