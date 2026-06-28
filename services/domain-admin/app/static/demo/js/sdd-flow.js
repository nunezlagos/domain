/*
 * sdd-flow.js — Componente de Workflows para la card del dashboard.
 *
 * Autónomo y desacoplado: no depende de focus.js / charts.js / portal.js.
 * Se monta sobre [data-sdd-flow] y decide su vista según el estado de la card:
 *   - card normal (chica)   -> vista LISTA (stepper vertical)
 *   - card .hero (centrada)  -> vista GRAFO estilo n8n + selector de workflows
 * Al hacer click en un paso se abre un POPOVER con el detalle real (tool MCP,
 * output y tabla que escribe), modelado a partir de domain-mcp.
 *
 * SRP: modelo (WORKFLOWS) · ListView · GraphView · Popover · Selector · Controller.
 * Todo encapsulado en un IIFE: no expone nada al scope global.
 */
(function () {
  'use strict';

  /* ------------------------------------------------------------------ *
   *  Modelo: workflows reales de domain-mcp (fases + tool + output + tabla)
   * ------------------------------------------------------------------ */
  const WORKFLOWS = [
    {
      id: 'sdd', name: 'SDD Pipeline', icon: 'diagram-project', active: 5,
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
    {
      id: 'intake', name: 'Issue Intake', icon: 'inbox', active: 1,
      steps: [
        { id: 'submit',  name: 'Submit',  icon: 'paper-plane',       desc: 'Entrada externa desde jira / github / slack / email', tool: 'domain_intake_submit', out: 'payload (pending)', table: 'issue_intake_payloads' },
        { id: 'review',  name: 'Review',  icon: 'magnifying-glass',   desc: 'Triage del reviewer sobre los pendientes', tool: 'domain_intake_list_pending', out: 'reviewing', table: 'issue_intake_payloads' },
        { id: 'approve', name: 'Approve', icon: 'check-double',       desc: 'Aprueba la entrada y la convierte en issue', tool: 'domain_intake_approve', out: 'committed_issue_id', table: 'issue_intake_payloads' },
        { id: 'commit',  name: 'Commit',  icon: 'code-branch',        desc: 'Confirma el issue para entrar al pipeline SDD', tool: 'domain_issue_create_commit', out: 'issue committed', table: 'sdd_requirements' },
      ],
    },
    {
      id: 'flow', name: 'Flow / Cron', icon: 'bolt', active: 2,
      steps: [
        { id: 'create',   name: 'Create',   icon: 'diagram-next', desc: 'Define un DAG de pasos (agent / skill / http / condition)', tool: 'domain_flow_create', out: 'flow (spec DAG)', table: 'flows' },
        { id: 'schedule', name: 'Schedule', icon: 'clock',        desc: 'Programa ejecución periódica (cron 5 campos)', tool: 'domain_cron_create', out: 'cron', table: 'crons' },
        { id: 'run',      name: 'Run',      icon: 'play',         desc: 'Ejecuta el DAG en orden topológico', tool: 'domain_flow_run', out: 'flow_run', table: 'flow_runs' },
        { id: 'status',   name: 'Status',   icon: 'wave-square',  desc: 'Estado, outputs y prompts del run', tool: 'domain_flow_status', out: 'status · outputs', table: 'flow_runs' },
      ],
    },
  ];

  const ST = Object.freeze({ DONE: 'done', ACTIVE: 'active', PENDING: 'pending' });

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
  function stateAt(i, activeIdx) {
    return i < activeIdx ? ST.DONE : i === activeIdx ? ST.ACTIVE : ST.PENDING;
  }

  /* ------------------------------------------------------------------ *
   *  Popover de detalle (compartido por ambas vistas)
   * ------------------------------------------------------------------ */
  function Popover(host) {
    const pop = h('div', 'sdf-pop');
    pop.setAttribute('role', 'dialog');
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

      // posiciona bajo el ancla, relativo al host (con scroll)
      const hr = host.getBoundingClientRect();
      const ar = anchor.getBoundingClientRect();
      pop.classList.add('open');
      const pw = pop.offsetWidth, ph = pop.offsetHeight;
      let left = ar.left - hr.left + host.scrollLeft + ar.width / 2 - pw / 2;
      let top = ar.bottom - hr.top + host.scrollTop + 8;
      left = Math.max(6, Math.min(left, host.scrollWidth - pw - 6));
      if (top + ph > host.scrollTop + host.clientHeight && ar.top - hr.top - ph - 8 > 0) {
        top = ar.top - hr.top + host.scrollTop - ph - 8; // arriba si no cabe abajo
      }
      pop.style.left = left + 'px';
      pop.style.top = top + 'px';
    }

    function toggle(step, state, anchor) {
      if (openId === step.id) { close(); return; }
      openFor(step, state, anchor);
    }
    return { toggle, close, el: pop };
  }

  /* ------------------------------------------------------------------ *
   *  Vista LISTA — stepper vertical (card chica)
   * ------------------------------------------------------------------ */
  function ListView(host, pop) {
    let rows = [];
    function mount(wf) {
      const ul = h('ul', 'sdf-list');
      rows = wf.steps.map((step, i) => {
        const li = h('li', 'sdf-row', { 'data-id': step.id, role: 'button', tabindex: '0' });
        li.innerHTML =
          '<span class="sdf-dot"></span>' +
          '<i class="sdf-ic fas fa-' + step.icon + '"></i>' +
          '<span class="sdf-name">' + step.name + '</span>' +
          '<span class="sdf-tag"></span>' +
          '<i class="sdf-chev fas fa-chevron-right"></i>';
        const onClick = () => pop.toggle(step, li.dataset.state, li);
        li.addEventListener('click', onClick);
        li.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onClick(); } });
        ul.appendChild(li);
        return li;
      });
      host.appendChild(ul);
    }
    function update(wf) {
      wf.steps.forEach((step, i) => {
        const st = stateAt(i, wf.active);
        rows[i].dataset.state = st;
        rows[i].querySelector('.sdf-tag').textContent = st === ST.ACTIVE ? step.desc : '';
      });
    }
    return { mount, update };
  }

  /* ------------------------------------------------------------------ *
   *  Vista GRAFO — nodos + bezier serpenteante (card centrada, estilo n8n)
   * ------------------------------------------------------------------ */
  const NODE_W = 138, NODE_H = 50, GAP_X = 46, GAP_Y = 34, PAD = 12;

  function GraphView(host, pop) {
    let canvas, svg, nodes = [], edges = [], pos = [], wf = null;

    function mount(workflow) {
      wf = workflow;
      canvas = h('div', 'sdf-graph');
      svg = s('svg', { class: 'sdf-edges' });
      canvas.appendChild(svg);
      nodes = wf.steps.map((step, i) => {
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

    function layout() {
      const width = host.clientWidth || 600;
      const cols = Math.max(1, Math.floor((width - PAD * 2 + GAP_X) / (NODE_W + GAP_X)));
      const rows = Math.ceil(wf.steps.length / cols);
      pos = wf.steps.map((_, i) => {
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
      update(wf);
    }

    function drawEdges() {
      svg.innerHTML = '';
      edges = [];
      for (let i = 0; i < wf.steps.length - 1; i++) {
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

    function update(workflow) {
      wf = workflow;
      wf.steps.forEach((_, i) => { if (nodes[i]) nodes[i].dataset.state = stateAt(i, wf.active); });
      edges.forEach((edge, i) => edge.classList.toggle('on', i + 1 <= wf.active));
    }
    return { mount, layout, update };
  }

  /* ------------------------------------------------------------------ *
   *  Controlador
   * ------------------------------------------------------------------ */
  function Controller(host) {
    const card = host.closest('.mosaic-item') || host.parentElement;
    let mode = null;          // 'list' | 'graph'
    let wfIdx = 0;            // workflow seleccionado
    let tickTimer = null;
    let pop = null, view = null, selectorEl = null;

    const wf = () => WORKFLOWS[wfIdx];
    const isHero = () => !!(card && card.classList.contains('hero'));

    function buildSelector() {
      const bar = h('div', 'sdf-tabs');
      WORKFLOWS.forEach((w, i) => {
        const b = h('button', 'sdf-tab' + (i === wfIdx ? ' on' : ''));
        b.innerHTML = '<i class="fas fa-' + w.icon + '"></i> ' + w.name;
        b.addEventListener('click', () => { if (i !== wfIdx) { wfIdx = i; rebuild(); } });
        bar.appendChild(b);
      });
      return bar;
    }

    function rebuild() {
      host.innerHTML = '';
      pop = Popover(host);
      if (mode === 'graph') {
        selectorEl = buildSelector();
        host.appendChild(selectorEl);
        view = GraphView(host, pop);
        view.mount(wf());
        view.layout();
      } else {
        view = ListView(host, pop);
        view.mount(wf());
        view.update(wf());
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
        const w = wf();
        w.active = w.active + 1 > w.steps.length - 1 ? 0 : w.active + 1;
        if (view) view.update(w);
      }, 2600);
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
