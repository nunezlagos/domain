"""HU-45.1: URL routing mínimo.

Una sola ruta `/` que devuelve un HTML placeholder.
HU-45.4 sumará las vistas reales (dashboard, members, usage, audit, tickets).
"""
from django.http import HttpResponse
from django.urls import path


def index(request):
    return HttpResponse(
        """<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <title>Domain Admin</title>
  <style>
    body { font-family: -apple-system, system-ui, sans-serif; margin: 0; padding: 4rem 1rem; background: #0f172a; color: #e2e8f0; }
    main { max-width: 640px; margin: 0 auto; text-align: center; }
    h1 { font-size: 2rem; margin: 0 0 1rem; }
    p { line-height: 1.5; opacity: 0.85; }
    code { background: #1e293b; padding: 0.15rem 0.4rem; border-radius: 4px; font-size: 0.9em; }
  </style>
</head>
<body>
  <main>
    <h1>Domain Admin</h1>
    <p>Django placeholder vivo (HU-45.1).</p>
    <p>Service: <code>domain-admin</code> &middot; Backend: <code>domain-mcp</code> &middot; Routing: Caddy</p>
    <p>Vistas reales entran en HU-45.2 y siguientes.</p>
  </main>
</body>
</html>""",
        content_type="text/html; charset=utf-8",
    )


urlpatterns = [
    path("", index, name="index"),
]