-- HU-01.6 local-dev-environment + HU-01.1 db-schema-migrations
-- Solo corre la primera vez que se inicializa el volumen domain_pg_data.
-- Para entornos no-Docker, golang-migrate aplica equivalentes en migration 000001.
-- IF NOT EXISTS hace este script idempotente; seguro re-ejecutar.

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
