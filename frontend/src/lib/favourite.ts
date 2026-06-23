import { PUBLIC_APPVIEW_URL } from '$env/static/public';

// Shared favourite/unfavourite calls against the appview, used by the collection
// header. `scope-missing` means the user's OAuth session predates the favourite
// scope and needs re-authorization (mirrors the follow flow).
export type FavouriteOutcome =
	| { status: 'ok'; uri: string }
	| { status: 'scope-missing' }
	| { status: 'error' };

export async function favouriteCollection(
	subjectUri: string,
	subjectCid: string
): Promise<FavouriteOutcome> {
	try {
		const res = await fetch(`${PUBLIC_APPVIEW_URL}/favourite`, {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ subjectUri, subjectCid })
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

export async function unfavouriteCollection(favouriteUri: string): Promise<boolean> {
	const rkey = favouriteUri.split('/').at(-1);
	try {
		const res = await fetch(`${PUBLIC_APPVIEW_URL}/favourite/${rkey}`, {
			method: 'DELETE',
			credentials: 'include'
		});
		return res.ok;
	} catch {
		return false;
	}
}
