-- migration: 000172_chat_ia_readonly_role (down)
-- description: rollback del rol domain_chat_reader.

-- Antes de dropear el rol hay que soltar TODA dependencia que tenga sobre
-- objetos del esquema. El up otorga SELECT/INSERT/UPDATE/DELETE sobre decenas
-- de tablas y la USAGE del schema public; revocar solo chat_* dejaba grants
-- colgando y el DROP ROLE fallaba con "cannot be dropped because some objects
-- depend on it". DROP OWNED BY revoca de una todos los privilegios concedidos
-- al rol (y borra lo que sea de su propiedad) en la base actual.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'domain_chat_reader') THEN
        DROP OWNED BY domain_chat_reader;
    END IF;
END $$;

DROP ROLE IF EXISTS domain_chat_reader;
