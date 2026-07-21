-- saver_count = number of DISTINCT authors with a save of this visual identity.
-- save_count keeps counting raw save records (still used for GC: save_count = 0).
-- The global feed ranks by saver_count so one user saving the same image into
-- many collections can't pin it to the top of everyone's feed.
ALTER TABLE visual_identity ADD COLUMN saver_count INTEGER NOT NULL DEFAULT 0 CHECK (saver_count >= 0);

-- The distinct count is recomputed from the save table on every change rather
-- than maintained incrementally: it's a small index scan (idx_save_vi) and
-- can't drift.
CREATE OR REPLACE FUNCTION trg_save_vi_count() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.visual_identity_id IS NOT NULL THEN
            UPDATE visual_identity SET
                save_count = save_count + 1,
                saver_count = (SELECT COUNT(DISTINCT author_did) FROM save WHERE visual_identity_id = NEW.visual_identity_id)
            WHERE id = NEW.visual_identity_id;
        END IF;
    ELSIF TG_OP = 'UPDATE' THEN
        IF OLD.visual_identity_id IS DISTINCT FROM NEW.visual_identity_id THEN
            IF OLD.visual_identity_id IS NOT NULL THEN
                UPDATE visual_identity SET
                    save_count = save_count - 1,
                    saver_count = (SELECT COUNT(DISTINCT author_did) FROM save WHERE visual_identity_id = OLD.visual_identity_id)
                WHERE id = OLD.visual_identity_id;
            END IF;
            IF NEW.visual_identity_id IS NOT NULL THEN
                UPDATE visual_identity SET
                    save_count = save_count + 1,
                    saver_count = (SELECT COUNT(DISTINCT author_did) FROM save WHERE visual_identity_id = NEW.visual_identity_id)
                WHERE id = NEW.visual_identity_id;
            END IF;
        END IF;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.visual_identity_id IS NOT NULL THEN
            UPDATE visual_identity SET
                save_count = save_count - 1,
                saver_count = (SELECT COUNT(DISTINCT author_did) FROM save WHERE visual_identity_id = OLD.visual_identity_id)
            WHERE id = OLD.visual_identity_id;
        END IF;
    END IF;
    RETURN NULL;
END;
$$;

UPDATE visual_identity vi SET saver_count = sub.cnt
FROM (
    SELECT visual_identity_id, COUNT(DISTINCT author_did) AS cnt
    FROM save
    WHERE visual_identity_id IS NOT NULL
    GROUP BY visual_identity_id
) sub
WHERE vi.id = sub.visual_identity_id;
