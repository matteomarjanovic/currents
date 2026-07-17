// Tracks whether the app has performed at least one client-side navigation this session.
// Once true, the browser's back button is guaranteed to land on a same-origin page — SvelteKit
// only pushes history entries for its own internal navigations — so `history.back()` is safe.
export const navHistory = $state({
	hasInternalHistory: false
});
