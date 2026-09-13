import type { Action } from 'svelte/action';

export const modeTabsState = $state({ organizeSidebarOpen: true });
export const mobileBottomBarState = $state({ hidden: false });

export const trackOrganizeSidebar: Action<HTMLElement> = (node) => {
	const sidebar = node.closest<HTMLElement>('[data-slot="sidebar"][data-state]');
	const sync = () => {
		modeTabsState.organizeSidebarOpen = sidebar?.dataset.state !== 'collapsed';
	};
	const observer = sidebar ? new MutationObserver(sync) : null;
	if (sidebar) observer?.observe(sidebar, { attributes: true, attributeFilter: ['data-state'] });
	sync();

	return {
		destroy() {
			observer?.disconnect();
		}
	};
};
