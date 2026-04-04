ALTER TABLE save
    DROP COLUMN IF EXISTS quality_score,
    DROP COLUMN IF EXISTS visual_identity_id;

DROP TABLE IF EXISTS visual_identity;
