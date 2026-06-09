DROP INDEX IF EXISTS idx_collection_parent;
ALTER TABLE collection DROP COLUMN IF EXISTS parent_uri;
