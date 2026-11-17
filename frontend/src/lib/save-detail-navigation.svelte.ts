// The floating trail outlives individual nested detail overlays.
export const detailNavigation = $state<{
	uri: string;
	imageAboveViewport: boolean;
	scrollRoot?: HTMLElement;
}>({ uri: '', imageAboveViewport: false });
