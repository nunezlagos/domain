"""HU-49.2: app Django del chat IA estilo NotebookLM.

Esta app NO migra: las tablas viven en domain-mcp (Go gestiona el DDL
via golang-migrate, ver mig 000171). Django solo lee/escribe via ORM
con `managed = False`. Sigue el mismo patron que el resto de los
mantenedores (agents, projects, etc).

Responsabilidades:
- admin_models: ORM models managed=False contra chat_conversations,
  chat_messages, chat_document_embeddings.
- views: endpoints REST del chat (HU-49.2) y pagina HTML (HU-49.3).
- retrieval: live-read de tablas del dominio + chunking + ranking.
- services: ChatService orquesta retrieval + LLM.
- prompts: system prompt + formato de contexto RAG.
- urls: ruteo.
"""
default_app_config = "chat.apps.ChatConfig"