from django.apps import AppConfig


class McpUptimeConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    name = "maintainers.mcpuptime"
    label = "mcpuptime"
    verbose_name = "Uptime del MCP"
