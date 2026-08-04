-- Pinterest's board pin_count, recorded when the job is queued so the progress
-- UI can report pins the listing stage never saw. Pinterest's logged-out board
-- feed silently omits ~10% of a board's pins, so listed < expected is normal
-- and needs surfacing rather than passing as a complete import.
-- 0 means "unknown" (section jobs and section-filtered board jobs import a
-- subset of the board by design, so the board's count is not their target).
ALTER TABLE import_job ADD COLUMN expected_count INT NOT NULL DEFAULT 0;
