ALTER TABLE import_job ADD COLUMN source_section_id   TEXT    NOT NULL DEFAULT '';
ALTER TABLE import_job ADD COLUMN filter_section_pins BOOLEAN NOT NULL DEFAULT false;
