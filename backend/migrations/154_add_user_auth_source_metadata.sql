-- Store lightweight request-source metadata for abuse investigation.
ALTER TABLE users ADD COLUMN IF NOT EXISTS registration_ip VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS registration_user_agent TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_ip VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_user_agent TEXT NOT NULL DEFAULT '';
