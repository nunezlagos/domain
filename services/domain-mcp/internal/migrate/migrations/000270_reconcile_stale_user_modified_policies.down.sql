-- restaura el flag is_user_modified a las policies reconciliadas por la up.
-- reversible: vuelve a marcarlas como editadas por operador.
UPDATE platform_policies
SET is_user_modified = true
WHERE slug IN ('agent-protocol', 'agent-voice');
