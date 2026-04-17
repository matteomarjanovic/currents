ALTER TABLE save
    ADD COLUMN content_nsid TEXT NOT NULL DEFAULT 'is.currents.content.image';

CREATE INDEX idx_save_content_nsid ON save (content_nsid);