-- migration: users_erasure
-- author: nunezlagos
-- issue: HU-23.4
-- description: agrega columnas is_erased + erased_at a users (GDPR Art. 17)
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_erased BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS erased_at TIMESTAMPTZ;

-- domain-lint-ignore-next: require-concurrent-index
-- reason: users tabla baja cardinalidad + index partial muy selectivo
CREATE INDEX IF NOT EXISTS users_is_erased_idx ON users(is_erased)
  WHERE is_erased = TRUE;
