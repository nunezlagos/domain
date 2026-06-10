// Domain dashboard JS — vanilla, sin framework. Refresh cada 15s.
async function fetchJSON(url) {
  const r = await fetch(url);
  if (!r.ok) throw new Error(`${url} → ${r.status}`);
  return r.json();
}

function renderStats(stats) {
  const grid = document.getElementById('stats-grid');
  const labels = {
    orgs: 'Organizaciones',
    projects: 'Proyectos',
    users: 'Usuarios',
    observations: 'Observaciones',
    knowledge_docs: 'Knowledge docs',
    agents: 'Agents',
    flows: 'Flows',
    skills: 'Skills',
    agent_runs_today: 'Agent runs hoy',
    flow_runs_today: 'Flow runs hoy',
  };
  grid.innerHTML = '';
  for (const [k, label] of Object.entries(labels)) {
    const v = stats[k];
    const card = document.createElement('div');
    card.className = 'stat';
    card.innerHTML = `<div class="label">${label}</div>
      <div class="value">${v < 0 ? '—' : v.toLocaleString('es-CL')}</div>`;
    grid.appendChild(card);
  }
}

function renderRuns(runs) {
  const body = document.getElementById('runs-body');
  body.innerHTML = '';
  if (!runs || !runs.length) {
    body.innerHTML = '<tr><td colspan="5" class="muted">Sin runs recientes.</td></tr>';
    return;
  }
  for (const r of runs) {
    const tr = document.createElement('tr');
    tr.innerHTML = `
      <td>${r.type}</td>
      <td><code>${r.id.slice(0, 8)}…</code></td>
      <td class="status-${r.status}">${r.status}</td>
      <td>${new Date(r.started_at).toLocaleString('es-CL')}</td>
      <td>${r.duration || '—'}</td>`;
    body.appendChild(tr);
  }
}

async function refresh() {
  try {
    const [stats, runs] = await Promise.all([
      fetchJSON('/admin/api/stats'),
      fetchJSON('/admin/api/recent-runs'),
    ]);
    renderStats(stats);
    renderRuns(runs);
    document.getElementById('last-refresh').textContent =
      'última actualización: ' + new Date().toLocaleTimeString('es-CL');
  } catch (e) {
    console.error(e);
    document.getElementById('last-refresh').textContent = 'error: ' + e.message;
  }
}

refresh();
setInterval(refresh, 15000);
