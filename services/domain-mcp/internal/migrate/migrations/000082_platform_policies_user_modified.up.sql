






ALTER TABLE platform_policies
  ADD COLUMN IF NOT EXISTS is_user_modified BOOLEAN NOT NULL DEFAULT FALSE;
