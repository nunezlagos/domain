-- Revertir: restaurar NOT NULL en api_keys.organization_id.
-- Nota: falla si existen filas con organization_id NULL (keys emitidas tras el up).
-- En una DB fresca (roundtrip) no hay NULLs, así que es seguro.
ALTER TABLE api_keys ALTER COLUMN organization_id SET NOT NULL;
