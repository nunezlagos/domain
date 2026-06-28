/*
 * sdd-flow.js — Workflows en curso por proyecto (card del dashboard).
 *
 * Cada proyecto corre un flujo (uno de los tipos reales de domain-mcp) y el
 * componente muestra su ESTADO: fase en curso, progreso y detalle por paso.
 *   - card normal (chica)   -> LISTA de proyectos con su flujo + estado
 *   - card .hero (centrada)  -> selector de proyectos + panel de estado + GRAFO n8n
 * Click en un paso del grafo abre un POPOVER con el detalle real (tool, output, tabla).
 *
 * Autónomo y desacoplado (IIFE, sin globals). No depende de focus.js: detecta el
 * estado hero por MutationObserver. SRP: catálogo · proyectos · vistas · popover · controlador.
 */
(function () {
  'use strict';

  /* ------------------------------------------------------------------ *
   *  Catálogo de flujos (tipos reales de domain-mcp) — pasos + tool + output + tabla
   * ------------------------------------------------------------------ */
  const WORKFLOWS = {
    sdd: {
      name: 'SDD Pipeline', icon: 'diagram-project',
      steps: [
        { id: 'explore', name: 'Explore', icon: 'magnifying-glass', desc: 'Analiza el prompt: intent, scope y módulos afectados', tool: 'domain_orchestrate', out: 'intent · scope · modules[]', table: '—' },
        { id: 'spec',     name: 'Spec',     icon: 'file-lines',       desc: 'Construye issue.md con escenarios Gherkin', tool: 'domain_issue_create_*', out: 'issue.md + Gherkin', table: 'sdd_requirements' },
        { id: 'propose',  name: 'Propose',  icon: 'lightbulb',        desc: 'Propuesta formal: scope y esfuerzo estimado', tool: 'domain_propose_*', out: 'proposal.md (draft)', table: 'sdd_proposals' },
        { id: 'design',   name: 'Design',   icon: 'compass-drafting', desc: 'Diseño técnico: ADRs, test_plan y sabotage_plan', tool: 'domain_mem_save', out: 'design.md · ADRs[]', table: 'sdd_designs' },
        { id: 'tasks',    name: 'Tasks',    icon: 'list-check',       desc: 'Descompone en tareas atómicas con dependencias', tool: 'domain_orchestrate_phase_result', out: 'tasks[]', table: 'issue_tasks' },
        { id: 'apply',    name: 'Apply',    icon: 'code',             desc: 'Implementa en código siguiendo TDD estricto', tool: 'domain_orchestrate_phase_result', out: 'files_changed[]', table: 'issue_code_references' },
        { id: 'verify',   name: 'Verify',   icon: 'shield-halved',    desc: 'Valida los escenarios Gherkin contra la implementación', tool: 'domain_verify_*', out: 'scenarios pass/fail', table: 'tdd_verifications' },
        { id: 'judge',    name: 'Judge',    icon: 'gavel',            desc: 'Sabotage tests: romper invariante → test rojo → restaurar', tool: 'domain_verify_update_item', out: 'sabotage_records[]', table: 'tdd_sabotage_records' },
        { id: 'review',   name: 'Review',   icon: 'user-shield',      desc: 'Contrasta la implementación contra políticas y skills', tool: 'domain_proposal_review', out: 'verdict · violations[]', table: 'tdd_verifications' },
        { id: 'archive',  name: 'Archive',  icon: 'box-archive',      desc: 'Marca el issue como implemented y actualiza el CHANGELOG', tool: 'domain_issue_set_status', out: 'archived', table: 'CHANGELOG.md' },
      ],
    },
    intake: {
      name: 'Issue Intake', icon: 'inbox',
      steps: [
        { id: 'submit',  name: 'Submit',  icon: 'paper-plane',       desc: 'Entrada externa desde jira / github / slack / email', tool: 'domain_intake_submit', out: 'payload (pending)', table: 'issue_intake_payloads' },
        { id: 'review',  name: 'Review',  icon: 'magnifying-glass',   desc: 'Triage del reviewer sobre los pendientes', tool: 'domain_intake_list_pending', out: 'reviewing', table: 'issue_intake_payloads' },
        { id: 'approve', name: 'Approve', icon: 'check-double',       desc: 'Aprueba la entrada y la convierte en issue', tool: 'domain_intake_approve', out: 'committed_issue_id', table: 'issue_intake_payloads' },
        { id: 'commit',  name: 'Commit',  icon: 'code-branch',        desc: 'Confirma el issue para entrar al pipeline SDD', tool: 'domain_issue_create_commit', out: 'issue committed', table: 'sdd_requirements' },
      ],
    },
    flow: {
      name: 'Flow / Cron', icon: 'bolt',
      steps: [
        { id: 'create',   name: 'Create',   icon: 'diagram-next', desc: 'Define un DAG de pasos (agent / skill / http / condition)', tool: 'domain_flow_create', out: 'flow (spec DAG)', table: 'flows' },
        { id: 'schedule', name: 'Schedule', icon: 'clock',        desc: 'Programa ejecución periódica (cron 5 campos)', tool: 'domain_cron_create', out: 'cron', table: 'crons' },
        { id: 'run',      name: 'Run',      icon: 'play',         desc: 'Ejecuta el DAG en orden topológico', tool: 'domain_flow_run', out: 'flow_run', table: 'flow_runs' },
        { id: 'status',   name: 'Status',   icon: 'wave-square',  desc: 'Estado, outputs y prompts del run', tool: 'domain_flow_status', out: 'status · outputs', table: 'flow_runs' },
      ],
    },
  };

  /* ------------------------------------------------------------------ *
   *  Proyectos con un flujo EN CURSO (instancias)
   * ------------------------------------------------------------------ */
  const PROJECTS = [
    { id: 'domain-mcp',   name: 'domain-mcp',   flow: 'sdd',    active: 5, since: 'hace 12 min', run: '#run-4812' },
    { id: 'domain-admin', name: 'domain-admin', flow: 'sdd',    active: 8, since: 'hace 3 min',  run: '#run-4813' },
    { id: 'saargo-cv',    name: 'saargo-cv',    flow: 'intake', active: 1, since: 'hace 1 min',  run: '#run-4814' },
    { id: 'licitalab',    name: 'licitalab',    flow: 'flow',   active: 2, since: 'hace 45 min', run: '#run-4815' },
  ];

  const ST = Object.freeze({ DONE: 'done', ACTIVE: 'active', PENDING: 'pending' });
  const wfOf = (proj) => WORKFLOWS[proj.flow];
  const stepsOf = (proj) => wfOf(proj).steps;
  const stateAt = (i, active) => (i < active ? ST.DONE : i === active ? ST.ACTIVE : ST.PENDING);
  const progressPct = (proj) => Math.round(((proj.active) / (stepsOf(proj).length - 1)) * 100);

  /* ------------------------------------------------------------------ *
   *  Util DOM
   * ------------------------------------------------------------------ */
  const SVG_NS = 'http://www.w3.org/2000/svg';
  function h(tag, cls, attrs) {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (attrs) for (const k in attrs) n.setAttribute(k, attrs[k]);
    return n;
  }
  function s(tag, attrs) {
    const n = document.createElementNS(SVG_NS, tag);
    if (attrs) for (const k in attrs) n.setAttribute(k, attrs[k]);
    return n;
  }

  /* ------------------------------------------------------------------ *
   *  Popover de detalle de un paso (solo en grafo)
   * ------------------------------------------------------------------ */
  function Popover(host) {
    const pop = h('div', 'sdf-pop', { role: 'dialog' });
    host.appendChild(pop);
    let openId = null;
    function close() { pop.classList.remove('open'); openId = null; }
    function openFor(step, state, anchor) {
      openId = step.id;
      pop.innerHTML =
        '<div class="sdf-pop-head">' +
          '<span class="sdf-pop-ic sdf-pop-ic--' + state + '"><i class="fas fa-' + step.icon + '"></i></span>' +
          '<div><b>' + step.name + '</b><span class="sdf-pop-state sdf-pop-state--' + state + '">' + state + '</span></div>' +
          '<button class="sdf-pop-x" aria-label="Cerrar"><i class="fas fa-xmark"></i></button>' +
        '</div>' +
        '<p class="sdf-pop-desc">' + step.desc + '</p>' +
        '<dl class="sdf-pop-meta">' +
          '<div><dt><i class="fas fa-wrench"></i> Tool</dt><dd><code>' + step.tool + '</code></dd></div>' +
          '<div><dt><i class="fas fa-arrow-right-from-bracket"></i> Output</dt><dd>' + step.out + '</dd></div>' +
          '<div><dt><i class="fas fa-database"></i> Tabla</dt><dd><code>' + step.table + '</code></dd></div>' +
        '</dl>';
      pop.querySelector('.sdf-pop-x').addEventListener('click', close);
      const hr = host.getBoundingClientRect();
      const ar = anchor.getBoundingClientRect();
      pop.classList.add('open');
      const pw = pop.offsetWidth, ph = pop.offsetHeight;
      let left = ar.left - hr.left + host.scrollLeft + ar.width / 2 - pw / 2;
      let top = ar.bottom - hr.top + host.scrollTop + 8;
      left = Math.max(6, Math.min(left, host.scrollWidth - pw - 6));
      if (top + ph > host.scrollTop + host.clientHeight && ar.top - hr.top - ph - 8 > 0) {
        top = ar.top - hr.top + host.scrollTop - ph - 8;
      }
      pop.style.left = left + 'px';
      pop.style.top = top + 'px';
    }
    function toggle(step, state, anchor) { if (openId === step.id) close(); else openFor(step, state, anchor); }
    return { toggle, close };
  }

  /* ------------------------------------------------------------------ *
   *  Vista LISTA — proyectos con su flujo en curso (card chica)
   * ------------------------------------------------------------------ */
  function ListView(host) {
    let rows = [];
    function mount() {
      const ul = h('ul', 'sdf-plist');
      rows = PROJECTS.map((proj) => {
        const li = h('li', 'sdf-prow', { 'data-id': proj.id });
        li.innerHTML =
          '<span class="sdf-pdot"></span>' +
          '<div class="sdf-pinfo">' +
            '<b>' + proj.name + '</b>' +
            '<span class="sdf-pmeta"><i class="fas fa-' + wfOf(proj).icon + '"></i> ' + wfOf(proj).name + ' · <span class="sdf-pphase"></span></span>' +
          '</div>' +
          '<div class="sdf-pprog"><span class="sdf-pprog-bar"><i></i></span><span class="sdf-pprog-num"></span></div>';
        ul.appendChild(li);
        return li;
      });
      host.appendChild(ul);
    }
    function update() {
      PROJECTS.forEach((proj, i) => {
        const steps = stepsOf(proj);
        const cur = steps[proj.active];
        const li = rows[i];
        li.dataset.state = proj.active >= steps.length - 1 ? 'done' : 'active';
        li.querySelector('.sdf-pphase').textContent = cur.name;
        li.querySelector('.sdf-pprog-bar i').style.width = progressPct(proj) + '%';
        li.querySelector('.sdf-pprog-num').textContent = (proj.active + 1) + '/' + steps.length;
      });
    }
    return { mount, update };
  }

  /* ------------------------------------------------------------------ *
   *  Vista GRAFO — panel de estado + grafo n8n del proyecto seleccionado
   * ------------------------------------------------------------------ */
  const NODE_W = 138, NODE_H = 50, GAP_X = 46, GAP_Y = 34, PAD = 12;

  function GraphView(host, pop) {
    let statusEl, canvas, svg, nodes = [], edges = [], pos = [], proj = null;

    function mount(project) {
      proj = project;
      statusEl = h('div', 'sdf-status');
      host.appendChild(statusEl);
      canvas = h('div', 'sdf-graph');
      svg = s('svg', { class: 'sdf-edges' });
      canvas.appendChild(svg);
      nodes = stepsOf(proj).map((step) => {
        const n = h('div', 'sdf-node', { 'data-id': step.id, role: 'button', tabindex: '0' });
        n.innerHTML =
          '<span class="sdf-node-ic"><i class="fas fa-' + step.icon + '"></i></span>' +
          '<span class="sdf-node-txt"><b>' + step.name + '</b><em>' + step.tool + '</em></span>';
        const onClick = (e) => { e.stopPropagation(); pop.toggle(step, n.dataset.state, n); };
        n.addEventListener('click', onClick);
        n.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onClick(e); } });
        canvas.appendChild(n);
        return n;
      });
      host.appendChild(canvas);
    }

    function renderStatus() {
      const steps = stepsOf(proj);
      const cur = steps[proj.active];
      const done = proj.active >= steps.length - 1;
      statusEl.innerHTML =
        '<span class="sdf-status-flow"><i class="fas fa-' + wfOf(proj).icon + '"></i> ' + wfOf(proj).name + '</span>' +
        '<span class="sdf-status-phase' + (done ? ' is-done' : '') + '">' +
          '<i class="fas fa-' + (done ? 'circle-check' : 'circle-notch fa-spin') + '"></i> ' + cur.name + '</span>' +
        '<span class="sdf-status-bar"><i style="width:' + progressPct(proj) + '%"></i></span>' +
        '<span class="sdf-status-meta">' + (proj.active + 1) + '/' + steps.length + ' · ' + proj.since + ' · <code>' + proj.run + '</code></span>';
    }

    function layout() {
      const width = host.clientWidth || 600;
      const steps = stepsOf(proj);
      const cols = Math.max(1, Math.floor((width - PAD * 2 + GAP_X) / (NODE_W + GAP_X)));
      const rows = Math.ceil(steps.length / cols);
      pos = steps.map((_, i) => {
        const row = Math.floor(i / cols);
        const within = i % cols;
        const col = row % 2 === 0 ? within : cols - 1 - within;
        return { x: PAD + col * (NODE_W + GAP_X), y: PAD + row * (NODE_H + GAP_Y), row };
      });
      nodes.forEach((n, i) => { n.style.transform = 'translate(' + pos[i].x + 'px,' + pos[i].y + 'px)'; });
      const totalW = PAD * 2 + cols * NODE_W + (cols - 1) * GAP_X;
      const totalH = PAD * 2 + rows * NODE_H + (rows - 1) * GAP_Y;
      canvas.style.height = totalH + 'px';
      svg.setAttribute('viewBox', '0 0 ' + totalW + ' ' + totalH);
      svg.setAttribute('width', totalW);
      svg.setAttribute('height', totalH);
      drawEdges();
      update(proj);
    }

    function drawEdges() {
      svg.innerHTML = '';
      edges = [];
      const steps = stepsOf(proj);
      for (let i = 0; i < steps.length - 1; i++) {
        const a = pos[i], b = pos[i + 1];
        let d;
        if (a.row === b.row) {
          const ltr = a.x < b.x;
          const x1 = a.x + (ltr ? NODE_W : 0), y1 = a.y + NODE_H / 2;
          const x2 = b.x + (ltr ? 0 : NODE_W), y2 = b.y + NODE_H / 2;
          const dx = (x2 - x1) * 0.5;
          d = 'M' + x1 + ',' + y1 + ' C' + (x1 + dx) + ',' + y1 + ' ' + (x2 - dx) + ',' + y2 + ' ' + x2 + ',' + y2;
        } else {
          const x1 = a.x + NODE_W / 2, y1 = a.y + NODE_H;
          const x2 = b.x + NODE_W / 2, y2 = b.y;
          const dy = (y2 - y1) * 0.5;
          d = 'M' + x1 + ',' + y1 + ' C' + x1 + ',' + (y1 + dy) + ' ' + x2 + ',' + (y2 - dy) + ' ' + x2 + ',' + y2;
        }
        svg.appendChild(s('path', { class: 'sdf-edge', d: d }));
        const flow = s('path', { class: 'sdf-edge-flow', d: d });
        svg.appendChild(flow);
        edges.push(flow);
      }
    }

    function update(project) {
      proj = project;
      stepsOf(proj).forEach((_, i) => { if (nodes[i]) nodes[i].dataset.state = stateAt(i, proj.active); });
      edges.forEach((edge, i) => edge.classList.toggle('on', i + 1 <= proj.active));
      renderStatus();
    }
    return { mount, layout, update };
  }

  /* ------------------------------------------------------------------ *
   *  Controlador
   * ------------------------------------------------------------------ */
  function Controller(host) {
    const card = host.closest('.mosaic-item') || host.parentElement;
    let mode = null, projIdx = 0, tickTimer = null, pop = null, view = null;
    const proj = () => PROJECTS[projIdx];
    const isHero = () => !!(card && card.classList.contains('hero'));

    function buildSelector() {
      const bar = h('div', 'sdf-tabs');
      PROJECTS.forEach((p, i) => {
        const b = h('button', 'sdf-tab' + (i === projIdx ? ' on' : ''));
        b.innerHTML = '<span class="sdf-tab-dot"></span> ' + p.name;
        b.addEventListener('click', () => { if (i !== projIdx) { projIdx = i; rebuild(); } });
        bar.appendChild(b);
      });
      return bar;
    }

    function rebuild() {
      host.innerHTML = '';
      pop = Popover(host);
      if (mode === 'graph') {
        host.appendChild(buildSelector());
        view = GraphView(host, pop);
        view.mount(proj());
        view.layout();
      } else {
        view = ListView(host);
        view.mount();
        view.update();
      }
    }

    function render() {
      const next = isHero() ? 'graph' : 'list';
      if (next === mode) return;
      mode = next;
      host.dataset.mode = mode;
      rebuild();
    }

    function startTicking() {
      stopTicking();
      tickTimer = setInterval(() => {
        // todos los flujos avanzan (simula ejecución en vivo)
        PROJECTS.forEach((p) => {
          const len = stepsOf(p).length;
          p.active = p.active + 1 > len - 1 ? 0 : p.active + 1;
        });
        if (view) view.update(proj());
      }, 2800);
    }
    function stopTicking() { if (tickTimer) { clearInterval(tickTimer); tickTimer = null; } }

    const mo = new MutationObserver(() => render());
    if (card) mo.observe(card, { attributes: true, attributeFilter: ['class'] });
    const ro = new ResizeObserver(() => { if (mode === 'graph' && view) view.layout(); });
    ro.observe(host);
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && pop) pop.close(); });
    host.addEventListener('click', (e) => { if (pop && e.target === host) pop.close(); });

    render();
    startTicking();
  }

  function init() {
    document.querySelectorAll('[data-sdd-flow]').forEach((host) => Controller(host));
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
