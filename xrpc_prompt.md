now let's add the save logic. you can find the needed lexicons in the @..\..\..\Documenti\robe\cu-so\lexicons\ folder. for the first implementation, let's avoid adding the image embeddings, let's keep it simple, we'll add that later.

the save and collection tables proposal is the following:

```sql
-- ============================================================
-- COLLECTION — an AT Protocol record, indexed locally
-- ============================================================
CREATE TABLE collection (
    uri             TEXT PRIMARY KEY,          -- AT-URI: at://did/is.currents.collection/tid
    author_did      TEXT NOT NULL REFERENCES "user"(did),
    name            TEXT NOT NULL,
    description     TEXT,
    created_at      TIMESTAMPTZ
);

CREATE INDEX idx_collection_author ON collection(author_did);


-- ============================================================
-- SAVE — a Currents AT Protocol record, indexed locally
-- ============================================================
CREATE TABLE save (
    uri               TEXT PRIMARY KEY,       -- AT-URI: at://did/is.currents.save/tid
    author_did        TEXT NOT NULL REFERENCES "user"(did),
    collection_uri    TEXT NOT NULL REFERENCES collection(uri),

    pds_blob_cid      TEXT NOT NULL,          -- blob CID on the user's PDS
    text              TEXT,                   -- optional caption / note
    origin_url        TEXT,                   -- source page URL, if web-clipped
    resave_of_uri      TEXT,                   -- AT-URI of the original save, if resaved

    created_at        TIMESTAMPTZ
);

CREATE INDEX idx_save_collection ON save(collection_uri);
CREATE INDEX idx_save_author ON save(author_did);
```
CREATE INDEX idx_save_vi ON save(visual_identity_id);
CREATE INDEX idx_save_vi_quality ON save(visual_identity_id, quality_score DESC);
```

All API endpoints follow the XRPC specification (https://atproto.com/specs/xrpc). This means:

Path format: All endpoints are served under /xrpc/{NSID}, where the NSID maps to a Lexicon definition. For example: /xrpc/is.currents.collection.getCollection.
Method types: Lexicon query methods map to HTTP GET (cacheable, no mutation). Lexicon procedure methods map to HTTP POST (may mutate state).
Parameters: Lexicon params are passed as URL query parameters on GET requests. Arrays use repeated parameter names. Booleans use true/false strings.
Request/response bodies: JSON with the standard atproto data model. Schemas defined by Lexicons.
Error responses: All errors return JSON { "error": "ErrorName", "message": "..." } with appropriate HTTP status codes, as specified by XRPC.
Cursors and pagination: List endpoints include a cursor parameter for pagination, following the XRPC convention where the cursor is opaque to the client and omitted from the first request.

make sure to use the http header atproto-proxy: Supports PDS service proxying. When a client sends a request through their PDS with an atproto-proxy header pointing to Currents' DID, the PDS forwards it with an inter-service JWT signed by the user's identity. The API service validates this JWT against the user's DID document signing key.

These are the XRPC endpoints the Currents AppView will expose, defined under the is.currents.* namespace:

Collections:

is.currents.collection.getCollection (query) — Get a collection by AT-URI, with save count and author info.
is.currents.collection.getCollections (query) — List collections for a given DID, with pagination.
is.currents.collection.createCollection (procedure) — Create a new collection with name, description.
is.currents.collection.updateCollection (procedure) — Update a collection's name or description.
is.currents.collection.deleteCollection (procedure) — Delete a collection by AT-URI.

Saves:

is.currents.save.getSave (query) — Get a save by AT-URI, hydrated with CDN image URL and metadata if available.
is.currents.save.getSaves (query) — List saves in a collection, with pagination.
is.currents.save.createSave (procedure) — Create a new save. The API writes the record to the user's PDS, runs the sync image pipeline, and indexes locally.
is.currents.save.updateSave (procedure) — Update a save's text, origin URL, or resave reference. Does not allow changing the associated collection.
is.currents.save.deleteSave (procedure) — Delete a save. Removes from PDS and local index, triggers canonical fallback if needed.