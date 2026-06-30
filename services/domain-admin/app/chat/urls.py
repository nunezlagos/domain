"""HU-49.2: URL routing del chat.

Pagina HTML + 6 endpoints REST. Se monta bajo /chat/ desde config.urls.
"""
from django.urls import path

from . import views

app_name = "chat"

urlpatterns = [
    path("", views.chat_page, name="page"),
    path("api/conversations", views.list_conversations, name="conversations_list"),
    path("api/conversations/new", views.create_conversation, name="conversations_new"),
    path(
        "api/conversations/<uuid:conversation_id>/messages",
        views.list_messages,
        name="messages_list",
    ),
    path(
        "api/conversations/<uuid:conversation_id>/messages/new",
        views.create_message,
        name="messages_new",
    ),
    path("api/messages/<int:message_id>", views.show_message, name="messages_show"),
    path(
        "api/conversations/<uuid:conversation_id>",
        views.delete_conversation,
        name="conversations_delete",
    ),
]