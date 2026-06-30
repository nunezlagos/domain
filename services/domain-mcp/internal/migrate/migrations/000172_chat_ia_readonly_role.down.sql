-- migration: 000172_chat_ia_readonly_role (down)
-- description: rollback del rol domain_chat_reader.

REVOKE ALL ON chat_conversations, chat_messages FROM domain_chat_reader;
REVOKE USAGE, SELECT ON SEQUENCE chat_messages_id_seq FROM domain_chat_reader;
DROP ROLE IF EXISTS domain_chat_reader;
