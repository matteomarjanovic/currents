DROP TABLE orphan_record;
DROP TABLE collection_delete_job;
DROP TRIGGER save_invalidate_collection_embedding ON save;
DROP FUNCTION invalidate_collection_embedding();
