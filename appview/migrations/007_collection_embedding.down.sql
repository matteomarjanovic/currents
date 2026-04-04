DROP INDEX IF EXISTS idx_collection_embedding;
ALTER TABLE collection DROP COLUMN IF EXISTS canonical_embedding;
