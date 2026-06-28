let activeSegment = null;
let searchFilter = '';
let modalMode = 'create';
let editingItem = null;

const view = document.getElementById('view');
const viewBackdrop = document.getElementById('viewBackdrop');
const viewTitle = document.getElementById('viewTitle');
const viewSubtitle = document.getElementById('viewSubtitle');
const viewBody = document.getElementById('viewBody');
const viewHeaderIcon = document.getElementById('viewHeaderIcon');
const viewBack = document.getElementById('viewBack');
const viewClose = document.getElementById('viewClose');

const modal = document.getElementById('modal');
const modalBackdrop = document.getElementById('modalBackdrop');
const modalClose = document.getElementById('modalClose');
const modalCancel = document.getElementById('modalCancel');
const modalForm = document.getElementById('modalForm');
const modalTitle = document.getElementById('modalTitle');

const profileCard = document.getElementById('profileCard');

function setViewHeader(seg) {
  viewHeaderIcon.style.color = seg.color;
  const svg = viewHeaderIcon.querySelector('svg');
  if (svg) svg.setAttribute('style', 'color: ' + seg.color);
  viewTitle.textContent = seg.label;
  viewSubtitle.textContent = SEGMENT_SUBTITLES[seg.id] || '';
}

function filterViewData(seg) {
  return (MOCK_DATA[seg.id] || []).filter(d => {
    if (!searchFilter) return true;
    const q = searchFilter.toLowerCase();
    return Object.values(d).some(v => String(v).toLowerCase().includes(q));
  });
}

function computeViewStats(data) {
  return {
    total: data.length,
    active: data.filter(d => d.status === 'active').length,
    totalUse: data.reduce((s, d) => s + (d.calls || d.uses || d.runs || 0), 0)
  };
}

function renderViewToolbar(seg) {
  return `
    <div class="view-toolbar">
      <div class="view-search">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
        </svg>
        <input type="text" id="viewSearchInput" placeholder="Buscar en ${seg.label}..." value="${searchFilter}" />
      </div>
      <button class="view-btn" id="refresh-btn">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>
        Refrescar
      </button>
      <button class="view-btn primary" id="addNew">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M12 5v14M5 12h14"/></svg>
        Nuevo
      </button>
    </div>`;
}

function renderViewStats(stats) {
  return `
    <div class="view-stats">
      <div class="view-stat glass">
        <div class="view-stat-label">Total</div>
        <div class="view-stat-value">${stats.total}</div>
      </div>
      <div class="view-stat glass">
        <div class="view-stat-label">Activos</div>
        <div class="view-stat-value" style="color: #4ade80">${stats.active}</div>
      </div>
      <div class="view-stat glass">
        <div class="view-stat-label">Inactivos</div>
        <div class="view-stat-value" style="color: var(--dj-text-muted)">${stats.total - stats.active}</div>
      </div>
      <div class="view-stat glass">
        <div class="view-stat-label">Uso 7d</div>
        <div class="view-stat-value">${stats.totalUse.toLocaleString()}</div>
      </div>
    </div>`;
}

function renderTableHeaders(seg) {
  const headers = {
    agents: '<th data-tooltip="Nombre del agente">Nombre</th><th data-tooltip="Slug unico (lowercase, guion)">Slug</th><th data-tooltip="Provider LLM (anthropic, openai, etc)">Provider</th><th data-tooltip="Modelo del provider (ej: claude-sonnet-4-5)">Modelo</th><th data-tooltip="Estado activo/inactivo">Estado</th><th data-tooltip="Cantidad de invocaciones">Calls</th>',
    skills: '<th data-tooltip="Nombre de la skill">Nombre</th><th data-tooltip="Slug unico (lowercase, guion)">Slug</th><th data-tooltip="Tipo (prompt/code/api/mcp_tool)">Tipo</th><th data-tooltip="Descripcion breve">Descripcion</th><th data-tooltip="Cantidad de invocaciones">Calls</th><th data-tooltip="Porcentaje de ejecuciones exitosas">Success</th>',
    flows: '<th data-tooltip="Nombre del flow">Nombre</th><th data-tooltip="Slug unico (lowercase, guion)">Slug</th><th data-tooltip="Cantidad de fases del flow">Fases</th><th data-tooltip="Estado activo/inactivo">Estado</th><th data-tooltip="Cantidad de ejecuciones">Runs</th>',
    prompts: '<th data-tooltip="Nombre del prompt">Nombre</th><th data-tooltip="Slug unico (lowercase, guion)">Slug</th><th data-tooltip="Modelo LLM target">Modelo</th><th data-tooltip="Estado activo/inactivo">Estado</th><th data-tooltip="Cantidad de veces usado">Usos</th>',
    projects: '<th data-tooltip="Nombre del proyecto">Nombre</th><th data-tooltip="Slug unico (lowercase, guion)">Slug</th><th data-tooltip="Estado activo/archivado">Estado</th><th data-tooltip="Cantidad de skills asociadas">Skills</th><th data-tooltip="Cantidad de agents">Agents</th><th data-tooltip="Cantidad de flows">Flows</th>',
    users: '<th data-tooltip="Email del usuario">Email</th><th data-tooltip="Rol (admin, operator, etc)">Rol</th><th data-tooltip="Estado activo/inactivo">Estado</th>',
    policies: '<th data-tooltip="Nombre de la policy">Nombre</th><th data-tooltip="Scope (platform/project/skill)">Scope</th><th data-tooltip="Tipo (security_rule, architecture, etc)">Kind</th><th data-tooltip="Estado activo/inactivo">Estado</th>',
    crons: '<th data-tooltip="Nombre del cron">Nombre</th><th data-tooltip="Expresion cron (5 fields)">Schedule</th><th data-tooltip="Estado activo/inactivo">Estado</th><th data-tooltip="Cuando se ejecuto por ultima vez">Ultimo run</th>',
  };
  return headers[seg.id] || '';
}

function renderTableRow(seg, row) {
  const id = row.slug || row.email || row.name;
  const statusBadge = `<span class="badge ${row.status === 'active' ? 'active' : 'inactive'}">${row.status}</span>`;
  let cells = '';

  if (seg.id === 'agents') {
    cells = `<td><b>${row.name}</b></td><td><code>${row.slug}</code></td><td>${row.provider}</td><td><code>${row.model}</code></td><td>${statusBadge}</td><td>${row.calls}</td>`;
  } else if (seg.id === 'skills') {
    cells = `<td><b>${row.name}</b></td><td><code>${row.slug}</code></td><td><span class="badge ${row.type}">${row.type.toUpperCase()}</span></td><td>${row.desc}</td><td>${row.calls}</td><td>${row.success}%</td>`;
  } else if (seg.id === 'flows') {
    cells = `<td><b>${row.name}</b></td><td><code>${row.slug}</code></td><td>${row.phases}</td><td>${statusBadge}</td><td>${row.runs}</td>`;
  } else if (seg.id === 'prompts') {
    cells = `<td><b>${row.name}</b></td><td><code>${row.slug}</code></td><td><code>${row.model}</code></td><td>${statusBadge}</td><td>${row.uses}</td>`;
  } else if (seg.id === 'projects') {
    cells = `<td><b>${row.name}</b></td><td><code>${row.slug}</code></td><td>${statusBadge}</td><td>${row.skills}</td><td>${row.agents}</td><td>${row.flows}</td>`;
  } else if (seg.id === 'users') {
    cells = `<td><b>${row.name}</b></td><td><span class="badge mcp">${row.role}</span></td><td>${statusBadge}</td>`;
  } else if (seg.id === 'policies') {
    cells = `<td><b>${row.name}</b></td><td><span class="badge prompt">${row.scope}</span></td><td><code>${row.kind}</code></td><td>${statusBadge}</td>`;
  } else if (seg.id === 'crons') {
    cells = `<td><b>${row.name}</b></td><td><code>${row.schedule}</code></td><td>${statusBadge}</td><td>${row.last_run}</td>`;
  }

  cells += `<td><div class="view-table-actions">
    <button class="view-table-btn" data-action="view" data-id="${id}" title="Ver">
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
    </button>
    <button class="view-table-btn" data-action="edit" data-id="${id}" title="Editar">
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.12 2.12 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
    </button>
    <button class="view-table-btn danger" data-action="delete" data-id="${id}" title="Eliminar">
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-2 14a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2L5 6"/></svg>
    </button>
  </div></td>`;
  return cells;
}

function renderViewEmpty(seg) {
  return `<div class="view-empty">
    <div style="font-size: 48px; margin-bottom: 16px; opacity: 0.5; font-family: 'Font Awesome 6 Free'; font-weight: 900;">${seg.icon}</div>
    <h3>Sin items</h3>
    <p>No hay items que coincidan con "${searchFilter}"</p>
    <button class="view-btn primary" id="addNewEmpty">+ Crear el primero</button>
  </div>`;
}

function wireViewEvents(seg) {
  const searchInput = document.getElementById('viewSearchInput');
  if (searchInput) {
    searchInput.addEventListener('input', (e) => {
      searchFilter = e.target.value;
      renderView(seg);
      setTimeout(() => {
        const el = document.getElementById('viewSearchInput');
        if (el) { el.focus(); el.setSelectionRange(el.value.length, el.value.length); }
      }, 0);
    });
  }
  document.getElementById('addNew')?.addEventListener('click', () => openModal(seg, 'create'));
  document.getElementById('addNewEmpty')?.addEventListener('click', () => openModal(seg, 'create'));
  document.getElementById('refresh-btn')?.addEventListener('click', () => showToast('Lista actualizada', 'success'));
  viewBody.querySelectorAll('[data-action]').forEach(btn => {
    btn.addEventListener('click', () => {
      const action = btn.dataset.action;
      const id = btn.dataset.id;
      if (action === 'view') {
        showToast('Viendo ' + id, 'success');
      } else if (action === 'edit') {
        const item = MOCK_DATA[seg.id].find(d => (d.slug || d.email || d.name) === id);
        openModal(seg, 'edit', item);
      } else if (action === 'delete') {
        if (confirm('¿Eliminar ' + id + '?')) {
          MOCK_DATA[seg.id] = MOCK_DATA[seg.id].filter(d => (d.slug || d.email || d.name) !== id);
          renderView(seg);
          showToast('Item eliminado', 'success');
        }
      }
    });
  });
}

function renderView(seg) {
  setViewHeader(seg);
  const data = filterViewData(seg);
  const stats = computeViewStats(data);
  let html = renderViewToolbar(seg);
  html += renderViewStats(stats);

  if (data.length > 0) {
    html += '<table class="view-table"><thead><tr>' + renderTableHeaders(seg) + '<th></th></tr></thead><tbody>';
    data.forEach(row => { html += '<tr>' + renderTableRow(seg, row) + '</tr>'; });
    html += '</tbody></table>';
  } else {
    html += renderViewEmpty(seg);
  }

  viewBody.innerHTML = html;
  wireViewEvents(seg);
}

function openSegment(seg) {
  if (activeSegment || focusActive) return;
  activeSegment = seg;
  searchFilter = '';
  renderView(seg);
  view._previouslyFocused = document.activeElement;
  view.setAttribute('aria-hidden', 'false');
  setTimeout(() => viewBackdrop.classList.add('active'), 100);
  setTimeout(() => {
    view.classList.add('active');
    if (viewBack && viewBack.focus) viewBack.focus();
  }, 350);
  setTimeout(() => {
    const search = document.getElementById('viewSearchInput');
    if (search && search.focus) search.focus();
  }, 700);
}

function closeView() {
  if (!activeSegment) return;
  if (view._previouslyFocused && view._previouslyFocused.focus) {
    try { view._previouslyFocused.focus(); } catch (e) {}
  }
  view.classList.remove('active');
  view.setAttribute('aria-hidden', 'true');
  viewBackdrop.classList.remove('active');
  setTimeout(() => { activeSegment = null; searchFilter = ''; }, 600);
}

function openModal(seg, mode, item) {
  modalMode = mode;
  editingItem = item;
  modalTitle.textContent = (mode === 'edit' ? 'Editar' : 'Nuevo') + ' ' + seg.label.slice(0, -1);
  modal._previouslyFocused = document.activeElement;
  modal.setAttribute('aria-hidden', 'false');
  modal.classList.add('active');
  modalBackdrop.classList.add('active');

  if (mode === 'edit' && item) {
    modalForm.name.value = item.name || '';
    modalForm.slug.value = item.slug || '';
    modalForm.description.value = item.desc || item.description || '';
    modalForm.status.value = item.status || 'active';
  } else {
    modalForm.reset();
  }
  setTimeout(() => { if (modalForm.name && modalForm.name.focus) modalForm.name.focus(); }, 150);
}

function closeModal() {
  if (modal._previouslyFocused && modal._previouslyFocused.focus) {
    try { modal._previouslyFocused.focus(); } catch (e) {}
  }
  modal.classList.remove('active');
  modalBackdrop.classList.remove('active');
  modal.setAttribute('aria-hidden', 'true');
  modalForm.reset();
  document.querySelectorAll('.form-row.has-error').forEach(r => r.classList.remove('has-error'));
}

function openProfile() {
  profileCard.classList.toggle('open');
}
