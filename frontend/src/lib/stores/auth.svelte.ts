export const auth = $state({
	user: null as { did: string; handle: string; displayName?: string; avatar?: string } | null,
	checked: false
});
