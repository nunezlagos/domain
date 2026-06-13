#!/bin/bash
# ============================================================================
# 02-roles.sh — least-privilege roles (.claude/rules/connection-pools.md)
#
# Ejecutado por docker-entrypoint-initdb.d en el primer boot del container.
# Las env vars APP_USER_PASSWORD y APP_ADMIN_PASSWORD se pasan via compose.
#
# Roles:
#   app_user     NOBYPASSRLS  → runtime queries con RLS enforced
#   app_admin    BYPASSRLS    → auth/audit lookups (org_id aún no conocido)
#   app_migrator NOBYPASSRLS  → golang-migrate, CREATE en schema public
# ============================================================================
set -euo pipefail

: "${APP_USER_PASSWORD:?env var APP_USER_PASSWORD requerida}"
: "${APP_ADMIN_PASSWORD:?env var APP_ADMIN_PASSWORD requerida}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-SQL
  DO \$\$
  BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
      CREATE ROLE app_user WITH LOGIN PASSWORD '$APP_USER_PASSWORD' NOBYPASSRLS;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_admin') THEN
      CREATE ROLE app_admin WITH LOGIN PASSWORD '$APP_ADMIN_PASSWORD' BYPASSRLS;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_migrator') THEN
      CREATE ROLE app_migrator WITH LOGIN PASSWORD '$APP_USER_PASSWORD' NOBYPASSRLS CREATEDB;
    END IF;
  END \$\$;

  GRANT CONNECT ON DATABASE "$POSTGRES_DB" TO app_user, app_admin, app_migrator;
  GRANT USAGE ON SCHEMA public TO app_user, app_admin, app_migrator;
  GRANT CREATE ON SCHEMA public TO app_migrator;

  ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
  ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
    GRANT ALL ON TABLES TO app_admin;
  ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO app_user, app_admin;
SQL

echo "domain-services: roles app_user, app_admin, app_migrator creados"
