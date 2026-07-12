<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar';
	import CanvasSidebar from '$lib/components/canvas/sidebar.svelte';
</script>

<!-- Bound the shell to the viewport (see organize/+page.svelte for the dvh
     rationale); the sidebar's library grid scrolls internally. The sidebar is
     wider than the default so the masonry gets two useful columns. -->
<Sidebar.Provider class="h-dvh overflow-hidden" style="--sidebar-width: 22rem">
	<CanvasSidebar />
	<Sidebar.Inset class="relative overflow-hidden">
		<!-- The canvas itself lands here later; for now the floating trigger
		     (which also opens the offcanvas sidebar on mobile) is the only chrome. -->
		<Sidebar.Trigger class="absolute top-3 left-3" />
	</Sidebar.Inset>
</Sidebar.Provider>
