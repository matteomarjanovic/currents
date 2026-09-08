ALTER TABLE user_pref
    ADD COLUMN last_save_removal_action TEXT NOT NULL DEFAULT 'ask'
    CHECK (last_save_removal_action IN ('ask', 'move-to-profile', 'delete'));
