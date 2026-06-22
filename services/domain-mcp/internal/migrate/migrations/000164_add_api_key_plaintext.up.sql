-- migration: add_api_key_plaintext
-- author: mnunez@saargo.com
-- issue: API keys visibles de nuevo en el dashboard (decision de producto)
-- description: agrega key_plaintext a auth_api_keys. El bcrypt (key_hash) se
--   conserva — el MCP lo necesita para autenticar (Resolve verifica bcrypt). El
--   plaintext se guarda en una columna aparte para poder mostrar la key de nuevo
--   en el detalle del mantenedor (con boton de copiar). TRADEOFF de seguridad
--   asumido explicitamente por el owner: la key queda recuperable en claro. Es
--   plataforma interna single-org. Endurecer a futuro: cifrado at-rest (pgcrypto)
--   en lugar de texto plano.
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE auth_api_keys ADD COLUMN key_plaintext TEXT;
