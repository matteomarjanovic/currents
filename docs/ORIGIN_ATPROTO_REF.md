# AT Protocol save provenance

`is.currents.feed.save` has an optional `originAtprotoRef` using
`com.atproto.repo.strongRef`: `{ "uri": "at://did:…/collection/rkey", "cid": "…" }`.
The URI identifies the source record; the CID identifies its version when the
image was saved. It is the **record CID**, not the image blob CID.

This is generic to AT Protocol, not restricted to Bluesky posts. It records
where the image was saved from, not authorship or licensing. `originUrl`
remains the clickable web source, attribution stays in `content.attribution`,
and `resaveOf` identifies the Currents save being resaved. These fields can
coexist.

## Creating a save (extension integration)

Add an optional multipart field to the existing authenticated `POST /api/save`
(also available as `POST /save`):

```ts
form.append('originAtprotoRef', JSON.stringify({ uri: post.uri, cid: post.cid }));
```

Supply a complete record URI with a DID authority and a valid CIDv1. Invalid
references return HTTP 400 before the handler uploads an image or writes a
record. Omit the field when the source is unknown; existing clients need no
changes. The rest of the upload flow and the successful redirect are unchanged.

The caller resolves the source record and supplies both values. The appview
validates their syntax without fetching the source or replacing its CID with
the latest version. The reference remains valid provenance if the source is
later edited or deleted. It does not verify that the referenced record contains
the saved image. The extension should associate each image with its containing
post, including in feeds, threads, and quoted posts; page-level URLs alone are
insufficient.

### Browser extension: Bluesky and compatible clients

The extension detects post images on `bsky.app`, `mu.social`, `witchsky.app`,
`northsky.app`, `blacksky.community`, `anisota.net`, and `tokimeki.blue` for right-click saves,
hover-button saves, and the multi-image picker. These clients share Bluesky post
record schemas. Each image retains its own candidate post links and CDN or
direct PDS `getBlob` identity. Anisota's card/quote boundaries are handled
explicitly so embedded quote links cannot hide the enclosing post's permalink. In the
worker, the Bluesky adapter resolves handles and fetches candidate posts through
the client's public AppView: Bluesky for Bluesky/Witchsky/Anisota/Tokimeki, Bluesky with a Eurosky
fallback for Mu, and Blacksky for Northsky/Blacksky.
It adds provenance only when the image's repository DID and blob CID match an
image embedded in the source record. Quoted images reference the quoted record;
images accompanying a quote reference the enclosing record. `originUrl` points
to the verified owning post on the client where the image was saved, including
when saving from a feed or thread. Cross-client permalinks are recognized too.

Tokimeki exposes each timeline post and embedded quote's URI in `data-aturi`.
Detection uses the nearest such container instead of neighbouring column links.
Expanded viewer images can match their blob identity to a mounted post image;
multiple possible source records leave the reference unknown. Media thumbnails
without a record container or a unique mounted source remain unattributed.

Avatars, banners, videos, and external-link preview thumbnails do not acquire
a post reference. Unknown ownership, deleted/unavailable posts, and lookup
failures leave the reference absent and do not prevent saving. Lookup requests
are public and send no credentials. Image download URLs are unchanged.

### Browser extension: Grain

On `grain.social`, detection identifies the selected photo's repository DID and
blob CID from its signed CDN URL and the containing gallery's permalink. This
works in gallery feed cards and on gallery detail pages. The worker queries
`social.grain.unspecced.getGallery` without credentials and matches the image to
a `social.grain.photo` view. Its photo URI and **photo record CID** become the
strong reference; neither the gallery record nor the gallery-item membership
record is used. Different images in a gallery retain independent references.
`originUrl` stays on the containing Grain gallery.

Avatars, unavailable galleries, unmatched photos, and ambiguous duplicate photo
records leave the reference absent. Saving the image still proceeds normally.
This adapter covers public gallery routes; private/shared-pool routes that do
not expose a public gallery reference remain unattributed.

### Browser extension: Pckt

Public articles on `pckt.blog` and its blog subdomains expose a published post's
`atproto_uri` and `atproto_cid` in their Inertia page data. The adapter matches
the selected image against that post's cover or image content before capturing
the complete reference. It uses the current history page state during client
navigation, and rejects page data belonging to a different URL. The rendered
page's record CID is retained rather than replaced with a later version.

Feed-card images can resolve through the linked article's public page data,
fetched by the worker without credentials. Pckt media originals and conversions
match by their shared media ID; download URLs remain unchanged. Multiple images
in one article reference the same owning document (`site.standard.document`, or
legacy `blog.pckt.document`), with the article URL as their web source.

Avatars, website previews, unattached uploads, unpublished/syncing/failed posts,
and missing references stay unattributed. Mini notes and custom-domain blogs
without this supported article metadata are not resolved by this adapter.

### Browser extension: Offprint

Public articles on `offprint.app` and its publication subdomains expose the
published `site.standard.document` URI and record CID in their Inertia page
data. The adapter matches covers, inline images, image grids, and carousels
before capturing that reference. It uses the canonical document reference,
not the secondary `app.offprint.document.article` reference, and retains the
rendered version during client navigation.

Feed images resolve through their linked public article without credentials.
CDN originals and conversions match by media ID without changing download URLs.
Avatars, link previews, unrelated media, drafts, and unsynced articles remain
unattributed. Missing metadata and lookup failures do not prevent saving.
Custom-domain publications without a supported Offprint host are not resolved.

### Browser extension: Leaflet

Leaflet images expose their repository DID and blob CID in `/api/atproto_images`
URLs. Article metadata identifies the document URI; feed and embedded-post
cards supply their own article links. Detection covers `leaflet.pub`, its
subdomains, and custom-domain publications exposing Leaflet's AT metadata.

In Leaflet's client-side reader, the URL and head metadata remain on the feed.
Detection reads the document URI from the opened article's main page DOM ID,
then performs the same image-to-record verification. Reader saves use Leaflet's
`/p/{did}/{rkey}` article permalink as their web source. Switching articles uses
the newly rendered page ID, including when the feed cards are unmounted. Missing
document IDs remain unattributed rather than falling back to stale head metadata.

The worker resolves the author's DID to their public PDS and fetches the source
`site.standard.document` or legacy `pub.leaflet.document` record without
credentials. It uses that record's CID only when the selected blob matches a
cover or a Leaflet image block, including nested linear/canvas content. Avatars,
website preview images, unrelated blobs, and unavailable records stay
unattributed. The web source remains the containing article, and image download
URLs are unchanged. Unlike the Pckt/Offprint snapshots, the reference identifies
the version verified on the PDS during the metadata lookup.

`image-origin.ts` defines the shared hint/result types, and
`clipper.content/detect-image-origin.ts` dispatches DOM detection to the
Bluesky, Grain, Pckt, Offprint, and Leaflet adapters. Source resolution lives in
`lib/bluesky-origin.ts`, `lib/grain-origin.ts`, `lib/pckt-origin.ts`, and
`lib/offprint-origin.ts`, and `lib/leaflet-origin.ts`.
Other Atmosphere apps can add adapters using the same
per-image transport and multipart field, without changing the save record flow.

### Image alt text

The adapters also return dedicated alt text for the matched image, when present.
Bluesky-compatible clients use the embedded image's `alt`; Grain uses the photo
view's `alt`, never its rendered gallery-title/caption fallback. Leaflet uses the
matched image block's `alt` (a cover alone has no dedicated alt field). Pckt uses
image-content `attrs.alt`, excluding its upload-filename default; Offprint uses
single-image `attrs.altText` or media `alt_text`, and per-item media `alt_text`
for grids/carousels. Captions, titles, article summaries, and arbitrary DOM alt
attributes are never used as fallbacks.

Single mode starts the lookup when the form mounts; multi mode starts one per
selected image. The field is editable, limited to 2,000 characters, and never
overwritten after the user types or clears it. Saving waits for that image's
pending lookup, then reuses the resolved source record/version. An empty or
explicitly cleared field stays empty; the worker does not refill it at save
time. Unknown sources and failed lookups leave the field empty without blocking
saving. The paused generic suggestion UI remains paused.

## Editing a save

Authenticated `PUT /api/save/{rkey}/origin-atproto-ref` accepts JSON:

```json
{"originAtprotoRef":{"uri":"at://did:plc:example/app.bsky.feed.post/key","cid":"bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku"}}
```

Use `{"originAtprotoRef":null}` to remove it. Omitting the field is HTTP 400.
The endpoint edits only the viewer's record, including resaves, and returns
`{"uri":"at://…/is.currents.feed.save/…","originAtprotoRef":…}` (null on removal).
It preserves other fields and uses `swapRecord` to reject concurrent changes.

The frontend displays this field only in Organize's image details sidebar.
Owners can add, edit, or remove it; another author's reference is read-only.

## Indexing and inheritance

Migration 058 adds nullable URI/CID columns to `save`, without a foreign key to
the source or an extra index. Provenance belongs to each save, independently
of visual identity: the same image may be saved from different posts.

TAP (and the shared ingestion path) indexes additions, replacements, and removals.
All XRPC save views expose the optional strong reference. PDS writes appear in
the index asynchronously, as with other save metadata. Existing saves have no
reference and need no backfill. Alt, attribution, label, and collection edits
preserve it. Currents resaves inherit it, which also preserves it through moves
and copies that use the resave flow. Deleting the source never deletes a save.
