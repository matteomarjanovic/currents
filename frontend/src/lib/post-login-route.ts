import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { share } from './stores/share.svelte';

// Where a native login lands. Normally the feed — but a share that arrived without a usable
// session is parked in the store (see routes/(without-navbar)/share/+page.svelte), and
// dropping the user on the feed would silently discard what they were trying to save.
//
// Navigates rather than returning a path so `resolve()` stays at the goto call site, which
// is what svelte/no-navigation-without-resolve checks for.
export function gotoAfterLogin(): Promise<void> {
	return share.pending
		? goto(resolve('/(without-navbar)/share'))
		: goto(resolve('/(with-navbar)/explore'));
}
