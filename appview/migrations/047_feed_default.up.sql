ALTER TABLE feed_pref
    ADD COLUMN default_feed TEXT NOT NULL DEFAULT 'personal'
        CHECK (default_feed IN ('general', 'new-worlds', 'personal'));
