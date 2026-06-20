"""HU-47.1: login simple con credenciales master via env vars.

Flujo:
- GET  /             → si autenticado redirect /dashboard/; sino muestra login
- POST /login/       → check ADMIN_EMAIL + ADMIN_PASSWORD env vars
                       OK:   set session["authenticated"]=True, redirect /dashboard/
                       MAL:  muestra error
- GET  /dashboard/   → si session OK muestra dashboard; sino redirect /login/
- GET  /logout/      → session.flush() + redirect /login/

Sesiones: signed cookie (default Django), sin DB.
CSRF: exempted en login (single-user, demo). Para multi-user, agregar template
      con {% csrf_token %}.

NOTA DE SEGURIDAD: ADMIN_EMAIL y ADMIN_PASSWORD vienen de env vars. Defaults
hardcoded para dev/demo. En producción setear en /opt/services/.env.
"""
import os

from django.http import HttpResponse, HttpResponseRedirect
from django.urls import path
from django.views.decorators.csrf import csrf_exempt


def _admin_creds() -> tuple[str, str]:
    """Lee credenciales de env vars con defaults para dev."""
    return (
        os.environ.get("ADMIN_EMAIL", "admin@admin.com"),
        os.environ.get("ADMIN_PASSWORD", "q1w2e3r4"),
    )


_LOGIN_HTML = """<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <title>Domain Admin - Login</title>
  <style>
    * {{ box-sizing: border-box; }}
    body {{
      font-family: -apple-system, system-ui, sans-serif;
      background: #0f172a;
      color: #e2e8f0;
      min-height: 100vh;
      margin: 0;
      display: flex;
      align-items: center;
      justify-content: center;
    }}
    form {{
      background: #1e293b;
      padding: 2.5rem;
      border-radius: 8px;
      width: 100%;
      max-width: 420px;
      box-shadow: 0 10px 25px rgba(0,0,0,0.3);
    }}
    h1 {{ margin: 0 0 0.5rem; font-size: 1.5rem; }}
    .sub {{ color: #94a3b8; font-size: 0.85rem; margin: 0 0 1.5rem; }}
    label {{ display: block; margin-bottom: 1rem; font-size: 0.9rem; }}
    input {{
      width: 100%;
      padding: 0.6rem;
      margin-top: 0.25rem;
      background: #0f172a;
      color: #e2e8f0;
      border: 1px solid #334155;
      border-radius: 4px;
      font-size: 1rem;
    }}
    input:focus {{ outline: 2px solid #2563eb; border-color: transparent; }}
    button {{
      width: 100%;
      padding: 0.75rem;
      margin-top: 0.5rem;
      background: #2563eb;
      color: white;
      border: none;
      border-radius: 4px;
      font-size: 1rem;
      font-weight: 600;
      cursor: pointer;
    }}
    button:hover {{ background: #1d4ed8; }}
    .error {{
      background: #7f1d1d;
      color: #fee2e2;
      padding: 0.75rem;
      border-radius: 4px;
      margin-bottom: 1rem;
      font-size: 0.9rem;
    }}
  </style>
</head>
<body>
  <form method="post" action="/login/">
    <h1>Domain Admin</h1>
    <p class="sub">Iniciá sesión para continuar</p>
    {error_block}
    <label>Email
      <input type="email" name="email" required autofocus autocomplete="username">
    </label>
    <label>Password
      <input type="password" name="password" required autocomplete="current-password">
    </label>
    <button type="submit">Entrar</button>
  </form>
</body>
</html>"""


_DASHBOARD_HTML = """<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <title>Domain Admin - Dashboard</title>
  <style>
    * {{ box-sizing: border-box; }}
    body {{
      font-family: -apple-system, system-ui, sans-serif;
      background: #0f172a;
      color: #e2e8f0;
      margin: 0;
      padding: 0;
    }}
    header {{
      background: #1e293b;
      padding: 1rem 2rem;
      display: flex;
      justify-content: space-between;
      align-items: center;
      border-bottom: 1px solid #334155;
    }}
    h1 {{ margin: 0; font-size: 1.2rem; }}
    h1 span {{ color: #60a5fa; }}
    a {{ color: #60a5fa; text-decoration: none; }}
    a:hover {{ text-decoration: underline; }}
    main {{ padding: 2rem; max-width: 960px; margin: 0 auto; }}
    .card {{
      background: #1e293b;
      padding: 1.5rem;
      border-radius: 8px;
      margin-bottom: 1rem;
    }}
    .grid {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; }}
    .stat {{ background: #1e293b; padding: 1.5rem; border-radius: 8px; text-align: center; }}
    .stat .n {{ font-size: 2rem; font-weight: 700; color: #60a5fa; }}
    .stat .l {{ color: #94a3b8; font-size: 0.85rem; margin-top: 0.25rem; }}
    ul {{ line-height: 1.8; padding-left: 1.25rem; }}
    code {{ background: #0f172a; padding: 0.15rem 0.4rem; border-radius: 4px; font-size: 0.85em; }}
  </style>
</head>
<body>
  <header>
    <h1>Domain <span>Admin</span></h1>
    <div>
      <span style="color: #94a3b8; font-size: 0.85rem; margin-right: 1rem;">{email}</span>
      <a href="/logout/">Logout</a>
    </div>
  </header>
  <main>
    <h2 style="margin-top: 0;">Dashboard</h2>
    <p style="color: #94a3b8;">Login OK. Próximas HUs: members, usage, audit, tickets, cost.</p>

    <div class="card">
      <strong>Estado del stack</strong>
      <ul style="margin-top: 0.5rem;">
        <li>MCP backend: <code>domain-mcp:8000</code></li>
        <li>Database: <code>postgres:5432</code></li>
        <li>Storage: <code>minio:9000</code></li>
        <li>Reverse proxy: <code>caddy:80</code></li>
      </ul>
    </div>

    <h3 style="margin-top: 2rem;">HUs abiertas</h3>
    <div class="grid">
      <div class="stat"><div class="n">0</div><div class="l">Members</div></div>
      <div class="stat"><div class="n">0</div><div class="l">Usage events</div></div>
      <div class="stat"><div class="n">0</div><div class="l">Audit logs</div></div>
      <div class="stat"><div class="n">0</div><div class="l">Tickets</div></div>
    </div>
  </main>
</body>
</html>"""


@csrf_exempt
def login_view(request):
    # Si ya está autenticado, mandarlo al dashboard.
    if request.session.get("authenticated"):
        return HttpResponseRedirect("/dashboard/")

    error_block = ""
    if request.method == "POST":
        email = request.POST.get("email", "").strip()
        password = request.POST.get("password", "")
        admin_email, admin_password = _admin_creds()
        if email == admin_email and password == admin_password:
            # Regen session ID para evitar fixation.
            request.session.cycle_key()
            request.session["authenticated"] = True
            request.session["email"] = admin_email
            return HttpResponseRedirect("/dashboard/")
        error_block = '<div class="error">Email o password incorrecto.</div>'

    return HttpResponse(_LOGIN_HTML.format(error_block=error_block))


def dashboard(request):
    if not request.session.get("authenticated"):
        return HttpResponseRedirect("/login/")
    email = request.session.get("email", "admin")
    return HttpResponse(_DASHBOARD_HTML.format(email=email))


def logout_view(request):
    request.session.flush()
    return HttpResponseRedirect("/login/")


urlpatterns = [
    path("", login_view, name="home"),
    path("login/", login_view, name="login"),
    path("dashboard/", dashboard, name="dashboard"),
    path("logout/", logout_view, name="logout"),
]