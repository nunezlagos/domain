-- Revierte 000184: quita la FK y la columna created_by de skill_executions.
ALTER TABLE skill_executions DROP CONSTRAINT IF EXISTS skill_executions_created_by_fkey;
ALTER TABLE skill_executions DROP COLUMN IF EXISTS created_by;
