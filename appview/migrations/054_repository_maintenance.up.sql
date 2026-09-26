-- Deletion intent must survive restarts and PDS rate limits. Keep completed
-- jobs as tombstones until TAP has removed the indexed collection too.
CREATE TABLE collection_delete_job (
    collection_uri TEXT PRIMARY KEY,
    owner_did TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX ON collection_delete_job (owner_did);

-- Age the broken reference, not the record: an old save can become orphaned
-- seconds ago during an ordinary cascade or out-of-order TAP delivery.
CREATE TABLE orphan_record (
    uri TEXT PRIMARY KEY,
    owner_did TEXT NOT NULL,
    target_uri TEXT NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON orphan_record (owner_did);

-- Keep invalidation in the same transaction as the save mutation. In
-- particular, an empty collection must stop recommending immediately, even
-- when a deletion comes from outside Currents or no debounce timer survives.
CREATE FUNCTION invalidate_collection_embedding() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF OLD.collection_uri = NEW.collection_uri AND OLD.visual_identity_id IS NOT DISTINCT FROM NEW.visual_identity_id THEN
            RETURN NULL;
        END IF;
    END IF;
    UPDATE collection SET canonical_embedding = NULL WHERE uri = OLD.collection_uri;
    IF TG_OP <> 'DELETE' THEN
        UPDATE collection SET canonical_embedding = NULL WHERE uri = NEW.collection_uri;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER save_invalidate_collection_embedding
AFTER DELETE OR UPDATE OF collection_uri, visual_identity_id ON save
FOR EACH ROW EXECUTE FUNCTION invalidate_collection_embedding();

-- Old empty collections can retain their last medoid after save deletions.
UPDATE collection c SET canonical_embedding = NULL
WHERE canonical_embedding IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM save s JOIN visual_identity vi ON vi.id = s.visual_identity_id
                  WHERE s.collection_uri = c.uri AND vi.embedding IS NOT NULL);
