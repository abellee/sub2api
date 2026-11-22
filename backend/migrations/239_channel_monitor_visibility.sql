-- User-facing channel status visibility:
--   all      = every logged-in user
--   selected = only the user IDs in channel_monitor_visible_user_ids (default)
-- Admins are never restricted by this setting. An empty allow-list means
-- only admins see the user-facing channel status surface.
INSERT INTO settings (key, value)
VALUES ('channel_monitor_visibility', 'selected')
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value)
VALUES ('channel_monitor_visible_user_ids', '[]')
ON CONFLICT (key) DO NOTHING;
