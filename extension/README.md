# Save to Currents

Browser extension for [Currents](https://currents.is). Save images to your
collections from any website:

- **One image** — right-click it and choose **Save to Currents**, or use the
  Currents button on a Pinterest or Instagram photo, to pick one or more collections and save.
- **A whole page** — click the toolbar icon, or right-click anywhere on the page
  and choose **Save images from this page…**. Pick images from the grid, add
  optional details to individual images, then select one or more collections.
  Scroll the page while the panel is open to discover newly loaded images.
- **Image button** — hover over an image on a selected website to save it. The
  button starts enabled on Pinterest. In the panel's Settings, add other domains
  or turn it on for every website. The button stays hidden on currents.is and
  Instagram videos.

Built with [WXT](https://wxt.dev) + Svelte, Manifest V3.

On Bluesky, Mu, Witchsky, Northsky, Blacksky, Anisota, and Tokimeki, post images automatically
retain the source AT Protocol record's URI and record CID when their ownership can be
verified, including quoted images and images picked from feeds. The source link
stays on the client you used. Unknown sources are saved without a reference.
See [AT Protocol save provenance](../ORIGIN_ATPROTO_REF.md).

Grain gallery photos also retain their individual `social.grain.photo` record
reference when verified. Their source link points to the containing gallery.

Pckt article covers and content images retain the published document's record
reference when matched to its page data, including saves from linked feed cards.

Offprint covers, inline images, grids, and carousels also retain their published
`site.standard.document` reference, including images saved from publication feeds.

Leaflet article images and feed covers retain their document reference after
matching the selected image to the author's public PDS record. Standard and
legacy Leaflet documents are supported, including custom-domain publications.

For these supported AT Protocol apps, dedicated alt text from the matched image
prefills its editable details field in both single and multi-image saves.
Captions, filenames, and generic page alt attributes are not copied. Editing or
clearing the field takes precedence over any pending source lookup.

Alt-text regression checks: after building the extension and installing the
frontend dependencies, run `node tests/alt-autofill.browser.mjs` from `extension/`.
These use mocked pages, public metadata responses, and save endpoints.
`node tests/leaflet-reader.browser.mjs` covers Leaflet's client-side reader,
including changing articles without navigating away from the feed URL.

## Build from source

Prerequisites: Node.js ≥ 20 (built and tested with 24.x) and npm.

```sh
npm install
npm run zip          # Chrome  → .output/save-to-currents-<version>-chrome.zip
npm run zip:firefox  # Firefox → .output/save-to-currents-<version>-firefox.zip
```

Unpacked builds are written to `.output/chrome-mv3/` and `.output/firefox-mv3/`.

## Configuration

No configuration is required: the production build targets the live Currents
services (`https://currents.is` and `https://api.currents.is`) by default, so a
clean `npm install && npm run zip` reproduces the published extension exactly.

Provenance resolver tests use Node.js 22.6 or newer:

```sh
node --experimental-strip-types --test tests/*.test.mjs
```

Local development overrides the endpoints via `.env.development` (see
`.env.example`); `npm run dev` runs against that config with hot reload.

## View transitions

Internal navigation uses `useViewNavigation()` from `src/lib/view-navigation.ts`.
Pass the state change to `navigate(() => ...)`, with `-1` for Back or Cancel.
The shared transition slides, fades, and blurs an inert snapshot of the outgoing
view while keeping live form state mounted. It preserves scroll positions,
resizes the panel with the content, and skips animation for reduced motion.
