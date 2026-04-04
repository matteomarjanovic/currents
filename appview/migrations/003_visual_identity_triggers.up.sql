CREATE OR REPLACE FUNCTION trg_save_vi_count() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.visual_identity_id IS NOT NULL THEN
            UPDATE visual_identity SET save_count = save_count + 1 WHERE id = NEW.visual_identity_id;
        END IF;
    ELSIF TG_OP = 'UPDATE' THEN
        IF OLD.visual_identity_id IS DISTINCT FROM NEW.visual_identity_id THEN
            IF OLD.visual_identity_id IS NOT NULL THEN
                UPDATE visual_identity SET save_count = save_count - 1 WHERE id = OLD.visual_identity_id;
            END IF;
            IF NEW.visual_identity_id IS NOT NULL THEN
                UPDATE visual_identity SET save_count = save_count + 1 WHERE id = NEW.visual_identity_id;
            END IF;
        END IF;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.visual_identity_id IS NOT NULL THEN
            UPDATE visual_identity SET save_count = save_count - 1 WHERE id = OLD.visual_identity_id;
        END IF;
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER trg_save_vi_count
AFTER INSERT OR UPDATE OF visual_identity_id OR DELETE ON save
FOR EACH ROW EXECUTE FUNCTION trg_save_vi_count();
