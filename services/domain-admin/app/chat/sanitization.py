"""HU-49.2 extension: SanitizationService — defense in depth.

Capa de seguridad que opera EN TORNO al LLM, no dentro. El prompt del
sistema es la primera linea de defensa; este servicio es la segunda
cuando el LLM es engañado por un prompt injection.

Responsabilidades:
1. **Pre-input**: detectar queries que piden generar codigo, ejecutar
   acciones, o que matchean patrones de prompt injection conocidos.
   En esos casos, retorna una respuesta segura sin llamar al LLM.
2. **Post-output**: censurar informacion sensible que el LLM pueda
   haber alucinado o leak-eado (emails, UUIDs internos, secret keys).

Patterns detectados (anti-injection):
- "generame un script/codigo/email/carta..."
- "escribime un codigo que..."
- "actua como / pretendes ser..."
- "ignore previous instructions / system prompt"
- "olvida todo / nueva instruccion"
- "ejecuta / corre / borra / elimina X"

Patterns censurados (privacy):
- emails completos (excepto admin@admin.com que es publico)
- UUIDs v4 (formato xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
- API keys (formato sk-...)
- API keys domk_...
- "Bearer XXXX..." tokens
"""
from __future__ import annotations

import re
from dataclasses import dataclass


@dataclass(frozen=True)
class SanitizationResult:
    """Resultado de la sanitizacion."""

    is_safe: bool
    reason: str = ""
    sanitized_text: str = ""


_CODE_GENERATION_PATTERNS = (
    "generame un script",
    "generame codigo",
    "generame una funcion",
    "generame un email",
    "generame una carta",
    "generame un texto",
    "generame un poema",
    "generame un parrafo",
    "escribime un script",
    "escribime codigo",
    "escribime una funcion",
    "escribime un email",
    "escribime una carta",
    "hazme un script",
    "hazme codigo",
    "hazme una funcion",
    "dame codigo",
    "dame un script",
    "dame un ejemplo de codigo",
    "dame el codigo",
    "programame",
    "desarrollame",
    "implementame",
    "codeme",
)

_ACTION_REQUEST_PATTERNS = (
    "ejecuta esto",
    "corre este comando",
    "borra todo",
    "elimina todo",
    "borrar base de datos",
    "drop table",
    "drop database",
    "delete from",
    "update users set",
    "rm -rf",
    "haz un deploy",
    "deploy a",
    "cambia la password",
    "cambia el password",
    "resetea",
    "limpia el cache",
    "borra el cache",
    "manda un email a",
    "enviale un email",
    "spamea",
)

_INJECTION_PATTERNS = (
    "ignore previous instructions",
    "ignore las instrucciones",
    "ignora las instrucciones",
    "olvida todo",
    "olvida las instrucciones",
    "olvida el system prompt",
    "nueva instruccion",
    "nuevas instrucciones",
    "system prompt",
    "actua como si fueras",
    "pretende ser",
    "pretend to be",
    "imagine you are",
    "jailbreak",
    "dan mode",
    "developer mode",
    "reveal your instructions",
    "show your system prompt",
    "what is your prompt",
    "cual es tu prompt",
    "cuales son tus instrucciones",
    "dame tu prompt",
)


_EMAIL_RE = re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b")
_UUID_RE = re.compile(
    r"\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b",
    re.IGNORECASE,
)
_API_KEY_RE = re.compile(r"\bsk-[a-zA-Z0-9_-]{20,}\b")
_DOMK_KEY_RE = re.compile(r"\bdomk_[a-zA-Z0-9_-]{10,}\b")
_BEARER_RE = re.compile(r"\bBearer\s+[A-Za-z0-9._-]{10,}\b")


def detect_code_generation(question: str) -> str | None:
    """Detecta si la query pide generar codigo/contenido.

    Returns el pattern que matcheo (para logging) o None.
    """
    q = question.lower().strip()
    for pattern in _CODE_GENERATION_PATTERNS:
        if pattern in q:
            return pattern
    return None


def detect_action_request(question: str) -> str | None:
    """Detecta si la query pide ejecutar una accion sobre el sistema.

    Returns el pattern que matcheo o None.
    """
    q = question.lower().strip()
    for pattern in _ACTION_REQUEST_PATTERNS:
        if pattern in q:
            return pattern
    return None


def detect_prompt_injection(question: str) -> str | None:
    """Detecta si la query intenta un prompt injection.

    Returns el pattern que matcheo o None.
    """
    q = question.lower().strip()
    for pattern in _INJECTION_PATTERNS:
        if pattern in q:
            return pattern
    return None


def censor_sensitive_text(text: str) -> tuple[str, int]:
    """Censura emails/UUIDs/api keys/bearer tokens en la respuesta del LLM.

    Returns (texto_censurado, cantidad_de_reemplazos).
    El admin@admin.com (email publico del login) NO se censura.
    """
    count = 0

    def censor_email(m: re.Match) -> str:
        email = m.group(0)
        if email == "admin@admin.com":
            return email
        nonlocal count
        count += 1
        return "[email censurado]"

    def replace_uuid(_m: re.Match) -> str:
        nonlocal count
        count += 1
        return "[uuid censurado]"

    def replace_api_key(_m: re.Match) -> str:
        nonlocal count
        count += 1
        return "[api-key censurada]"

    def replace_domk(_m: re.Match) -> str:
        nonlocal count
        count += 1
        return "[key censurada]"

    def replace_bearer(_m: re.Match) -> str:
        nonlocal count
        count += 1
        return "Bearer [token censurado]"

    text = _EMAIL_RE.sub(censor_email, text)
    text = _UUID_RE.sub(replace_uuid, text)
    text = _API_KEY_RE.sub(replace_api_key, text)
    text = _DOMK_KEY_RE.sub(replace_domk, text)
    text = _BEARER_RE.sub(replace_bearer, text)
    return text, count


def pre_check(question: str) -> SanitizationResult:
    """Pre-check de la query del usuario.

    Retorna SanitizationResult con is_safe=False si la query es peligrosa.
    El caller (ChatService) usa esto para NO llamar al LLM y responder
    con un mensaje de rechazo.
    """
    if not question or not question.strip():
        return SanitizationResult(is_safe=False, reason="Pregunta vacia")

    code_match = detect_code_generation(question)
    if code_match:
        return SanitizationResult(
            is_safe=False,
            reason=f"solicitud de generacion de codigo ({code_match!r})",
            sanitized_text=(
                "No genero codigo ni contenido arbitrario. Soy un asistente de "
                "consulta sobre el panel de administracion. Si necesitas codigo, "
                "preguntale a tu IDE o asistente de desarrollo. ¿Querés que te "
                "cuente algo sobre los datos del sistema?"
            ),
        )

    action_match = detect_action_request(question)
    if action_match:
        return SanitizationResult(
            is_safe=False,
            reason=f"solicitud de accion ({action_match!r})",
            sanitized_text=(
                "No puedo ejecutar acciones sobre el sistema (borrar, modificar, "
                "deploy, enviar emails, etc). Soy un asistente de solo-lectura. "
                "Para acciones, usa los mantenedores del panel o la API. "
                "¿Querés que te ayude a entender algo de los datos?"
            ),
        )

    injection_match = detect_prompt_injection(question)
    if injection_match:
        return SanitizationResult(
            is_safe=False,
            reason=f"prompt injection ({injection_match!r})",
            sanitized_text=(
                "Disculpa, no puedo cambiar mi comportamiento ni revelar mis "
                "instrucciones. Estoy aca para responder sobre el panel de "
                "administracion. ¿En que te ayudo?"
            ),
        )

    return SanitizationResult(is_safe=True)