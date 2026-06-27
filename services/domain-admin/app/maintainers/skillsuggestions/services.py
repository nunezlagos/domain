"""HU-52.3: logica de la app skillsuggestions (LLM-as-judge, human-in-the-loop).

DECISION DE INTEGRACION (lee esto antes de tocar nada):

  LECTURA  -> ORM directo (managed=False) contra `skill_suggestions`.
              Es el mismo patron que el resto del admin (DB compartida,
              single-tenant). List/Get/CountPending no tienen efectos.

  ESCRITURA (approve/reject/apply) -> se DELEGA al endpoint REST del
              domain-mcp (Go) server-to-server. NO se replica por ORM.

  Por que NO replicar la transicion por ORM (como hizo feedback con el voto):
    - El service Go es la UNICA fuente de verdad de la transicion: aplica
      guards optimistas (pending->approved, approved->applied, anti
      doble-apply), escribe audit_log con payload_hash SHA-256, y el APPLY
      muta `skills` transaccionalmente (split/merge/refine/archive), incluso
      llamando al LLM para refine. Reimplementar todo eso en Django seria
      fragil y duplicaria la auditoria (regla: un solo lugar audita).
    - feedback SI escribia por ORM porque era un upsert trivial sin audit ni
      logica de dominio. Aca la logica vive en Go: por eso se llama a Go.

  Como se evita exponer el Bearer al browser (igual objetivo que feedback):
    - El browser habla SOLO con Django (sesion del admin + CSRF).
    - Django reenvia a Go server-to-server con un Bearer de entorno
      (DOMAIN_API_TOKEN), que NUNCA llega al browser.

  Degradacion (regla dura 7 / robustez): si DOMAIN_API_TOKEN o la URL base
  no estan configurados, o el Go esta caido, la transicion devuelve un error
  claro (SuggestionApiError) y la view muestra un mensaje; no crashea.
"""
from __future__ import annotations

import json
import logging
import urllib.error
import urllib.request

from django.conf import settings

from .models import SkillSuggestion

log = logging.getLogger(__name__)

VALID_KINDS = (
    SkillSuggestion.KIND_SPLIT,
    SkillSuggestion.KIND_MERGE,
    SkillSuggestion.KIND_REFINE,
    SkillSuggestion.KIND_ARCHIVE,
)
VALID_STATUSES = (
    SkillSuggestion.STATUS_PENDING,
    SkillSuggestion.STATUS_APPROVED,
    SkillSuggestion.STATUS_REJECTED,
    SkillSuggestion.STATUS_APPLIED,
)

_DEFAULT_BASE_URL = "http://domain-mcp:8080"
_DEFAULT_TIMEOUT = 15.0


# ── Errores ────────────────────────────────────────────────────────────────
class SuggestionApiError(RuntimeError):
    """Falla al hablar con el endpoint REST del domain-mcp.

    `status` es el HTTP code del Go (o None si ni siquiera se pudo conectar);
    `code`/`message` salen del body de error del Go cuando esta disponible.
    """

    def __init__(self, message: str, *, status: int | None = None, code: str = ""):
        super().__init__(message)
        self.status = status
        self.code = code


class ApiNotConfiguredError(SuggestionApiError):
    """Falta DOMAIN_API_TOKEN o la URL base: no se puede transicionar."""


# ── Lectura (ORM directo) ────────────────────────────────────────────────────
def list_suggestions(
    *,
    status: str | None = None,
    kind: str | None = None,
    skill_slug: str | None = None,
    limit: int = 200,
) -> list[SkillSuggestion]:
    """Lista sugerencias para la vista admin, con filtros opcionales."""
    qs = SkillSuggestion.objects.all()
    if status in VALID_STATUSES:
        qs = qs.filter(status=status)
    if kind in VALID_KINDS:
        qs = qs.filter(kind=kind)
    if skill_slug:
        qs = qs.filter(skill_slug=skill_slug.strip())
    return list(qs.order_by("-created_at")[:limit])


def get_suggestion(suggestion_id) -> SkillSuggestion | None:
    """Detalle por id. Devuelve None si no existe."""
    return SkillSuggestion.objects.filter(id=suggestion_id).first()


def count_pending() -> int:
    """Conteo de pendientes (badge del sidebar). Read-only, barato."""
    return SkillSuggestion.objects.filter(
        status=SkillSuggestion.STATUS_PENDING
    ).count()


def kind_breakdown() -> list[dict]:
    """Resumen pendientes por kind (para el header de la lista)."""
    from django.db.models import Count

    rows = (
        SkillSuggestion.objects.filter(status=SkillSuggestion.STATUS_PENDING)
        .values("kind")
        .annotate(total=Count("id"))
        .order_by("-total")
    )
    return list(rows)


# ── Diff / payload para el detalle ───────────────────────────────────────────
def extract_diff(suggestion: SkillSuggestion) -> dict:
    """Normaliza el payload (forma segun kind) a algo presentable en el detalle.

    Devuelve un dict uniforme que el template recorre:
      {
        "kind": str,
        "summary": str,            # una linea humana
        "before": str | None,      # contenido actual (refine)
        "after": str | None,       # contenido propuesto (refine)
        "children": list[dict],    # hijos propuestos (split)
        "merge": dict | None,      # {targets, merged_content} (merge)
        "reason": str | None,      # motivo (archive)
        "raw": str,                # payload crudo formateado (siempre, fallback)
      }
    El payload viene de un LLM: puede faltar cualquier campo. Nunca asumimos
    forma; todo acceso es defensivo.
    """
    payload = suggestion.payload if isinstance(suggestion.payload, dict) else {}
    kind = suggestion.kind

    out = {
        "kind": kind,
        "summary": "",
        "before": None,
        "after": None,
        "children": [],
        "merge": None,
        "reason": None,
        "raw": json.dumps(suggestion.payload, indent=2, ensure_ascii=False, default=str),
    }

    if kind == SkillSuggestion.KIND_REFINE:
        out["before"] = payload.get("current_content") or payload.get("old_content")
        out["after"] = payload.get("new_content") or payload.get("content")
        out["summary"] = "Reescribir el contenido del skill."
    elif kind == SkillSuggestion.KIND_SPLIT:
        children = payload.get("children") or payload.get("skills") or []
        out["children"] = [c for c in children if isinstance(c, dict)]
        out["summary"] = f"Dividir en {len(out['children'])} skill(s) hijo(s)."
    elif kind == SkillSuggestion.KIND_MERGE:
        out["merge"] = {
            "targets": payload.get("targets")
            or payload.get("merge_with")
            or payload.get("slugs")
            or [],
            "merged_content": payload.get("merged_content") or payload.get("content"),
        }
        out["summary"] = "Consolidar con otro(s) skill(s)."
    elif kind == SkillSuggestion.KIND_ARCHIVE:
        out["reason"] = payload.get("reason") or payload.get("rationale")
        out["summary"] = "Archivar (soft-delete) el skill."

    return out


# ── Transicion: delega al domain-mcp (Go) ─────────────────────────────────────
def _base_url() -> str:
    base = (
        getattr(settings, "DOMAIN_API_BASE_URL", "")
        or getattr(settings, "DOMAIN_BASE_URL", "")
        or _DEFAULT_BASE_URL
    )
    return base.rstrip("/")


def _token() -> str:
    return getattr(settings, "DOMAIN_API_TOKEN", "") or ""


def _post(path: str, timeout: float = _DEFAULT_TIMEOUT) -> dict:
    """POST autenticado al domain-mcp. Devuelve el dict de respuesta (data)."""
    token = _token()
    if not token:
        raise ApiNotConfiguredError(
            "DOMAIN_API_TOKEN no configurado: no se puede operar sobre la "
            "sugerencia (la transicion vive en el domain-mcp)."
        )

    url = _base_url() + path
    req = urllib.request.Request(url, method="POST", data=b"")
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Content-Type", "application/json")

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read().decode("utf-8", errors="replace")
            return _parse_ok(body)
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace") if exc.fp else ""
        code, msg = _parse_err(detail)
        log.warning(
            "skill-suggestion POST %s -> HTTP %s code=%s", path, exc.code, code or "-"
        )
        raise SuggestionApiError(
            msg or f"el domain-mcp devolvio HTTP {exc.code}",
            status=exc.code,
            code=code,
        ) from None
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        log.warning("skill-suggestion POST %s -> sin conexion: %s", path, exc)
        raise SuggestionApiError(
            f"no se pudo contactar al domain-mcp: {type(exc).__name__}",
            status=None,
        ) from None


def _parse_ok(body: str) -> dict:
    if not body:
        return {}
    try:
        parsed = json.loads(body)
    except (ValueError, TypeError):
        return {}
    if isinstance(parsed, dict) and "data" in parsed:
        return parsed["data"] if isinstance(parsed["data"], dict) else {"data": parsed["data"]}
    return parsed if isinstance(parsed, dict) else {}


def _parse_err(body: str) -> tuple[str, str]:
    """Extrae (code, message) del body de error del Go ({error, message})."""
    if not body:
        return "", ""
    try:
        parsed = json.loads(body)
    except (ValueError, TypeError):
        return "", ""
    if not isinstance(parsed, dict):
        return "", ""
    return str(parsed.get("error", "")), str(parsed.get("message", ""))


def approve(suggestion_id) -> dict:
    """pending -> approved. NO aplica (regla dura 6)."""
    return _post(f"/api/v1/skill-suggestions/{suggestion_id}/approve")


def reject(suggestion_id) -> dict:
    """pending -> rejected."""
    return _post(f"/api/v1/skill-suggestions/{suggestion_id}/reject")


def apply(suggestion_id) -> dict:
    """approved -> applied. Muta `skills` en el Go (paso humano 2, separado)."""
    return _post(f"/api/v1/skill-suggestions/{suggestion_id}/apply")
