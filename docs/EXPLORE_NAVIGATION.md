# Explore detail history and performance

Opening a related image pushes a shallow-history entry. The trail navigates back
through those entries; swiping replaces only the current entry. History, loaded
related pages, and per-depth swipe sequences must survive Back navigation.

## Bounded rendering

Only the current detail and immediate predecessor render their full UI. This keeps
ordinary Back instantaneous. Earlier overlays retain the `SaveDetail` component's
state (including related items/cursor and image metadata), but suspend its markup.
Their image cards, dialogs, event handlers, image elements and masonry observers
are released. A height placeholder preserves the overlay's scroll range.

When an older layer re-enters the two-layer window, its UI is rebuilt from the
retained data without refetching related pages. Its saved scroll position is
applied after masonry's ResizeObserver/animation-frame layout. The desktop long
image pane also preserves its own scroll offset. Inactive detail observers and
pagination are gated by `active`, as is subscription to the active swipe sequence.

The navigation trail is a single layer above the detail stack. It stays mounted
while related images open. Its thumbnails keep the same aspect-ratio bounds,
expand within the space between the mode switcher and right controls, and use
continuous pointer interpolation for zoom.

This is a rendering bound, not a promise of constant total memory: history,
related data and swipe sequences still grow with the session. It avoids retaining
a full interactive page for every step. The shared grid also releases offscreen interactive cards, as described below;
Organize’s separate canvas remains a backlog item.

## Investigation, 2026-11-13

A controlled Playwright/Chromium run opened 40 related-image details, scrolling
each before entering the next. The fixture mocked API responses and SVG images;
these numbers isolate frontend overhead, not production image/network costs.
Measurements used Chrome DevTools Protocol after explicit garbage collection.

| At depth 40 | Before | After |
| --- | ---: | ---: |
| DOM nodes (browser counter) | 361,468 | 36,017 |
| JS event listeners | 30,287 | 1,397 |
| JavaScript heap | 370.8 MB | 43.0 MB |
| Cumulative browser task duration | 12.66 s | 2.44 s |

The original implementation retained every detail's complete DOM, including its
interactive related grid. `inert` and `aria-hidden` did not suspend those components.
Inactive details also retained pagination observers and subscribed to the active
swipe sequence. These costs accumulated with depth.

Regression coverage in `frontend/e2e/save-detail-swipe.spec.ts` checks the two-layer
rendering bound at 40 steps, preserved pages and 16,000px scroll restoration after
suspension on native iOS/Android layouts, swipe sequence restoration, trail jumps,
continuous hover, and uninterrupted trail DOM when opening a related image.


## Shared-grid rendering, 2026-11-16

Desktop save controls mount on pointer entry or keyboard focus, rather than for
every card near the viewport. Their first reveal fades in. Cards without a login
also wait for interaction before mounting their login/save control.

The shared grid retains the masonry library's lightweight aspect-ratio frames for
all loaded items. An IntersectionObserver rooted in the nearest scrolling ancestor
keeps interactive cards within 800px of that viewport. Cards leaving that region
are unmounted again. Frames preserve page height and ordering; loaded data and
swipe sequences remain independent of whether the card is mounted. This bounds
interactive component/image DOM, not the number of sizing frames or stored items.

Focus and busy callbacks keep an offscreen card alive while its picker, create
collection dialog, context menu, report dialog, mobile drawer, or optimistic save
is active. The initial 20 eager related thumbnails stay ready while the grid is
at its start; deep scrolling releases those as well. Initial server markup stays
intact until the first visibility observation.

The masonry container uses `overflow: clip`. `overflow: hidden` allows focus and
`scrollIntoView` to set an invisible internal scroll offset, which can leave a
returned page blank even when window scroll is zero. The clipping container does
not become a scrolling ancestor.

A controlled Chromium development-build fixture loaded 1,000 saves, supplied 773
collections, visited cards in 100-item increments, and returned to the top. API
and SVG images were mocked; these figures describe frontend overhead, not real
image decoding/network cost. Heap samples followed explicit garbage collection.

| After traversal | Before | On-demand controls only | Controls + offscreen release |
| --- | ---: | ---: | ---: |
| Mounted cards | 521 | 521 | 54 |
| Live elements | 9,464 | 5,296 | 2,494 |
| CDP DOM nodes | 107,230 | 48,941 | 15,778 |
| JS event listeners | 6,355 | 5,313 | 643 |
| JavaScript heap | 124.6 MB | 58.4 MB | 28.5 MB |

After returning to the top, 35 cards were mounted, page height remained 55,257px,
and the grid's internal scroll offset was zero. A separate regression checks
first-hover and keyboard controls, context-menu access, and retaining an open
picker while the owning card is outside the render window. Existing tests cover
page append, suggestions, long press, related-image preloading and deep Back.

These results do not establish an additional benefit from replacing the bounded
two-detail window with one persistent panel. That remains a separate experiment
if real-use navigation still has measurable remaining cost after these changes.


## Trail preview interaction

The collapsed trail's thumbnail buttons are its hover targets. Extra space around
them is reserved for shadows and does not receive pointer input. Once expanded,
a narrow strip bridges the spaces between thumbnails. The existing hover-card
component shows a larger image below a thumbnail after 500ms. A 6px buffer around
the expanded strip keeps it open; leaving the strip closes the trail and preview,
even when moving onto the preview itself. Moderation preferences and GIF
playback preferences apply to the preview too.

Both thumbnail and preview use `SaveImage`, as the grid does. Standard images on
`cdn.currents.is/img/` receive Bunny image-optimizer resize/quality parameters and
responsive candidates. Other image hosts, including staging `dev.currents.is`,
are passed through. The larger preview is mounted only when its hover card opens.


## Organize Related browser

Organize's Related tab has its own local journey. Selecting a grid image replaces
its compact hero and related results inside the sidebar, keeping the Organize URL
and canvas selection in place. Back/Forward and the thumbnail trail restore prior
steps, including loaded pages/cursors and scroll positions. Starting a different
canvas selection starts a new journey. Switching between Details and Related keeps
the journey's data but releases the inactive Related markup, observers and shortcuts.

Only one interactive hero/grid renders, irrespective of journey depth. Older steps
retain data, not mounted grids. Like Explore, the first related request asks for 12
images and subsequent pages ask for 50. Pagination is paused while restoring layout
and scroll so a previous step's deep scroll cannot fetch pages for the next image.

The toolbar keeps Back/Forward only. Unmodified Left/Right keys navigate the main
Organize grid from Details or the root Related step, updating the real canvas
selection and keeping that tile in view. In a nested Related step, the keys instead
follow a snapshot of the parent related grid from which that image was opened,
replacing the current step as Explore's image arrows do. The Related context owns
both boundaries too, so reaching its first/last image cannot jump into the main grid.
Back/Forward traverse nested selections; opening another related image after going
Back discards the forward branch. Shortcuts yield to tabs, resize handles, text
fields, menus and dialogs. Android Back returns through the journey before the
parent handler closes the sidebar.

Changing the right sidebar width reflows the main masonry grid. Pointer dragging
and keyboard resizing preserve the vertical offset of the top-left visible tile,
using the same anchor as closing the right sidebar or toggling the left one. A drag
keeps its initial anchor across width changes. Opening Details anchors the clicked
tile.

The shared Explore trail accepts an explicit local path and visit callback here.
Its vertical variant is anchored near the top of the Back and Forward row and
uses the same right inset as the Related grid. It includes the current image
once its hero scrolls above the panel
viewport. The first thumbnail stays at the stack's position as the rest expand
downward. The trail opens its
moderated image previews to the left. The horizontal Explore trail keeps using
shallow browser history and its existing layout.


### Organize on touchscreens

The top image in both tabs uses the same `swipe` action as Explore: horizontal
movement locks to image navigation, vertical movement remains ordinary scrolling,
two fingers yield to pinch, and touches starting in the 28px OS edge gutters are
ignored. The shared Organize image stage renders its neighbouring images eagerly
on mobile, follows the finger and completes/cancels the slide. Details and root
Related swipes select the main canvas; nested Related swipes keep the same parent
related-grid snapshot as keyboard navigation. The current image's native pinch
transfers its active pointers into image focus; moderation overlays retain control
of unrevealed images.

On mobile, the Related tab keeps its Back and Forward buttons above the safe-area
inset with 48px targets. The thumbnail trails in Explore and Organize Related are
desktop-only.
