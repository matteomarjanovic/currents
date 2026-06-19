-- Partial index for an actor's unsorted saves (collection_uri = ''), newest first.
-- Powers is.currents.feed.getUnsortedSaves / the profile "Unsorted" tab.
CREATE INDEX idx_save_unsorted ON save (author_did, created_at DESC) WHERE collection_uri = '';
