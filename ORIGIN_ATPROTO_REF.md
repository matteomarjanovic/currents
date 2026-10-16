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
insufficient. Extension implementation is deferred.

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
