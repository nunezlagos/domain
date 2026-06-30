from django.apps import AppConfig


class SkillSuggestionsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    name = "maintainers.skillsuggestions"
    label = "skillsuggestions"
    verbose_name = "Skill suggestions (LLM-as-judge)"
