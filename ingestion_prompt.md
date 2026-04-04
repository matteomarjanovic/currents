now let's work on the core of currents: the image ingestion. this should happen in the goroutine that listens to the websocket: basically, when we receive a new "save" event from the websocket, we currently just save the image reference to the db, in the save table. let's add a new idea: "visual identity". this will be a new table in the db that will identify images that are so similar they can practically be considered the same. this will allow us to avoid saving multiple copies of the same image, and also to group similar images together. when we receive a new "save" event, there can be different scenarios:
- the image is a resave of an existing image: in this case, we can just link the new save to the existing visual_identity - no need to compute its embedding and check if there is a similar visual_identity.
- the image is not a resave, but we can find the same CID in the db: in this case, we can also link the new save to the existing visual_identity - no need to compute its embedding and check if there is a similar visual_identity, since the CID is computed from the image content and guarantees uniqueness.
- the image is a new image and no matching CID is found: in this case, we need to compute its embedding and check if there is a similar visual_identity. if there is, we can link the new save to the existing visual_identity. if there isn't, we need to create a new visual_identity for this image and link the new save to it. we'll also compute a quality_score and the dominant_colors for the new image, to be used for placeholder rendering.

the visual_identity table will have the following columns:
```sql
-- ============================================================
-- VISUAL IDENTITY — the deduplicated "concept" of an image
-- ============================================================
CREATE TABLE visual_identity (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Canonical blob reference (the "best" version of this image, used for feed display and embedding)
    canonical_blob_did  TEXT,                   -- DID of the PDS hosting the best blob
    canonical_blob_cid  TEXT,                   -- CID of the best blob
                                                -- Both NULL = orphan (all sources deleted)

    embedding           VECTOR(768),            -- SigLIP 2 representation

    -- Engagement (Currents saves)
    save_count          INTEGER DEFAULT 0,

    dominant_colors     JSONB,                  -- extracted palette for placeholder rendering

    created_at          TIMESTAMPTZ DEFAULT NOW()
);

-- HNSW index for fast approximate nearest-neighbor search
CREATE INDEX idx_vi_embedding
    ON visual_identity USING hnsw (embedding vector_cosine_ops);

-- Partial index: exclude orphans from feed queries
CREATE INDEX idx_vi_active
    ON visual_identity (id)
    WHERE canonical_blob_did IS NOT NULL;
```

also, the following columns will be added to the save table:
```sql
ALTER TABLE save
    ADD COLUMN visual_identity_id UUID NOT NULL REFERENCES visual_identity(id);
    ADD COLUMN quality_score      REAL NOT NULL; -- resolution × aspect_ratio_weight, for ranking
    ADD COLUMN dominant_colors     JSONB;              -- extracted palette for placeholder rendering

CREATE INDEX idx_save_vi ON save(visual_identity_id);
CREATE INDEX idx_save_vi_quality ON save(visual_identity_id, quality_score DESC);
```

a proposal for the quality_score is:
```go
func qualityScore(width, height int) float64 {
    shortSide := math.Min(float64(width), float64(height))
    aspect := float64(width) / float64(height)

    // Resolution: smooth ramp between 200px and 600px
    resScore := math.Max(0.0, math.Min(1.0, (shortSide-200)/(600-200)))

    // Aspect ratio: full score between 0.5 and 2.0, penalize outside
    idealMin, idealMax := 0.5, 2.0
    var arScore float64
    if aspect >= idealMin && aspect <= idealMax {
        arScore = 1.0
    } else {
        distance := math.Max(idealMin-aspect, aspect-idealMax)
        arScore = math.Max(0.0, 1.0-distance)
    }

    result := (resScore + arScore) / 2
    return math.Round(result*1000) / 1000
}
```

the embedding will be computed using the SigLIP 2 model, which is a state-of-the-art image embedding model. the computation is done by a fastapi web server at /embed/image route (see @inference/main.py). the url of the server should be configurable with an env variable. we will use the canonical blob for computing the embedding, since it is the best version of the image and will be used for feed display and ranking. similarity ≥ 0.98 (cosine distance ≤ 0.02) will be considered a match for linking to an existing visual_identity.

An example Resolution Algorithm. When a new image arrives (from a Currents save) and the CID is not found in the db, we compute its embedding and perform a nearest neighbor search against existing visual_identities (use pgvector-go library https://github.com/pgvector/pgvector-go):
- Query pgvector for the nearest visual identity: SELECT id, embedding <=> $1 AS distance FROM visual_identity ORDER BY embedding <=> $1 LIMIT 1
- If similarity ≥ 0.98 (cosine distance ≤ 0.02): this is a near-duplicate.
  - The new provenance record (save or public_discovery) references the existing visual_identity.
  - Compare the new image's quality_score against the current canonical's quality. If the new image scores higher (better resolution, better aspect ratio), update canonical_blob_did and canonical_blob_cid.
  - Increment save_count (for Currents saves).
- If similarity < 0.98: this is a novel image.
  - Create a new visual_identity row with the embedding and initial scores.
  - Set canonical_blob_did / canonical_blob_cid from the new image.
  - The provenance record references the new visual_identity.

when an update or deletion on the save table is made, we will need to update visual_identity accordingly: if a save is deleted, we will need to decrement the save_count for the linked visual_identity and update the canonical_blob_did and canonical_blob_cid with the new best blob. And if the save_count reaches 0, we can consider the visual_identity as an orphan (no active saves linked to it) and set its canonical_blob_did and canonical_blob_cid to NULL.
to do this, we can use database triggers to automatically update the visual_identity table whenever there are changes to the save table. this way, we can ensure that the visual_identity table is always up-to-date and consistent with the saves in the system. we can create triggers for insert, update, and delete operations on the save table to handle all scenarios of linking and unlinking saves to visual identities and adding/removing from save_count.
we should evaluate what parts of the logic are more suitable to happen in the application layer (golang) and what parts can be efficiently handled in the database layer (postgres). for example, the embedding computation and nearest neighbor search will likely happen in the application layer, while the updates to visual_identity based on save changes can be efficiently handled with database triggers.

i hope now you have enough context. based on these thoughts from me, let's start planning the implementation for this feature! if you see some critical logical flaws or edge cases that i haven't considered, please point them out. also, if you have suggestions for improving the quality_score formula or the fact that we should store the canonical_blob info in the visual_identity record instead of retrieving the best blob dynamically (i'm still considering), i'm all ears! let's make sure we have a solid plan before we start coding.




# Verification

1. Migration: docker compose up --build applies 002 and 003 cleanly; psql -c "\d visual_identity" shows all columns and indexes.
2. Trigger: insert a save row with a known visual_identity_id, verify save_count increments; delete it, verify save_count decrements.
3. Novel image path: POST a new save to the HTTP handler → TAP fires → check visual_identity table has a new row with embedding and dominant_colors; save row has visual_identity_id set.
4. Resave path: create a second save with resave_of_uri pointing to the first → verify both saves share the same visual_identity_id without a new VI row being created.
5. Near-duplicate path: upload two visually near-identical images → verify they share one VI row (cosine distance check in the log or via SELECT embedding <=> … FROM visual_identity).
6. Inference down: stop inference server → POST a save → verify the save row exists with visual_identity_id = NULL and the TAP event was acked (no redeliver).
7. Canonical re-election: delete the save that is the canonical blob → verify visual_identity.canonical_blob_cid updates to the next-best save's CID, or goes NULL if no saves remain.